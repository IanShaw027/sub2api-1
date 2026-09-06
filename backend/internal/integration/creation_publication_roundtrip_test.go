//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type publicationRoundTripResolver struct {
	store service.MediaObjectStore
}

func (r publicationRoundTripResolver) Resolve(context.Context) (*service.MediaStorageBinding, service.MediaObjectStore, error) {
	return &service.MediaStorageBinding{ProfileID: "backup", Prefix: "roundtrip"}, r.store, nil
}

type publicationHTTPResult struct {
	status int
	header http.Header
	body   []byte
}

func publicationHTTPRequest(t *testing.T, client *http.Client, method, url, actor string, body io.Reader, headers map[string]string) publicationHTTPResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	require.NoError(t, err)
	if actor != "" {
		request.Header.Set("Authorization", "Bearer "+actor)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return publicationHTTPResult{status: response.StatusCode, header: response.Header, body: data}
}

func decodePublicationHTTP(t *testing.T, response publicationHTTPResult) service.CreationPublication {
	t.Helper()
	require.Equal(t, http.StatusOK, response.status, "%s", response.body)
	var envelope struct {
		Code int                         `json:"code"`
		Data service.CreationPublication `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.body, &envelope))
	require.Zero(t, envelope.Code)
	return envelope.Data
}

// The identity adapter below models an already-verified actor, not JWT issuance
// or session revocation. Everything after it uses production HTTP handlers,
// PostgreSQL repositories, publication service, and the actual encrypted S3 store.
func TestCreationPublicationHTTPPostgresS3RoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	postgresImage := strings.TrimSpace(os.Getenv("SUB2API_TEST_POSTGRES_IMAGE"))
	if postgresImage == "" {
		postgresImage = "postgres:18.1-alpine3.23"
	}
	pg, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase("publication_roundtrip"), tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, pg)
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, repository.ApplyMigrations(ctx, db))

	const accessKey = "publication-test"
	const secretKey = "publication-test-secret"
	const bucket = "publication-roundtrip"
	minio, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:RELEASE.2025-04-22T22-12-26Z",
			ExposedPorts: []string{"9000/tcp"},
			Cmd:          []string{"server", "/data"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     accessKey,
				"MINIO_ROOT_PASSWORD": secretKey,
				// A disposable 256-bit test key enables the production AES256 PutObject path.
				"MINIO_KMS_SECRET_KEY": "publication-test:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)),
			},
			WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, minio)
	require.NoError(t, err)
	host, err := minio.Host(ctx)
	require.NoError(t, err)
	port, err := minio.MappedPort(ctx, "9000/tcp")
	require.NoError(t, err)
	endpoint := "http://" + net.JoinHostPort(host, port.Port())
	inspector := s3.NewFromConfig(aws.Config{
		Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	_, err = inspector.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
	store, err := repository.NewS3MediaStoreFactory()(ctx, &service.BackupS3Config{
		Endpoint: endpoint, Region: "us-east-1", Bucket: bucket,
		AccessKeyID: accessKey, SecretAccessKey: secretKey, ForcePathStyle: true,
	})
	require.NoError(t, err)
	repo := repository.NewCreationPublicationRepository(db)
	h := handler.NewCreationPublicationHandler(service.NewCreationPublicationService(repo, publicationRoundTripResolver{store: store}))

	actors := map[string]int64{}
	for _, actor := range []string{"owner", "other"} {
		var id int64
		require.NoError(t, db.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
			actor+"@publication.test", "unused-in-identity-adapter").Scan(&id))
		actors[actor] = id
	}
	router := gin.New()
	router.GET("/api/v1/creation/gallery/:id/media", h.Media)
	private := router.Group("/api/v1/creation", func(c *gin.Context) {
		if id, ok := actors[strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")]; ok {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		}
		c.Next()
	})
	private.POST("/publications", h.Upload)
	private.GET("/publications/requests/:request_id", h.Status)
	private.DELETE("/publications/:id", h.Delete)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := server.Client()

	imageData := image.NewRGBA(image.Rect(0, 0, 8, 8))
	imageData.SetRGBA(3, 4, color.RGBA{R: 80, G: 150, B: 240, A: 255})
	var pngData bytes.Buffer
	require.NoError(t, png.Encode(&pngData, imageData))
	requestID := uuid.NewString()
	publicationPath := server.URL + "/api/v1/creation/publications"
	upload := func(actor, title string) publicationHTTPResult {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, value := range map[string]string{
			"request_id": requestID, "visibility": "public", "kind": "image", "title": title,
			"prompt": "Real HTTP and encrypted object storage", "model": "integration-model",
		} {
			require.NoError(t, writer.WriteField(key, value))
		}
		file, err := writer.CreateFormFile("file", "roundtrip.png")
		require.NoError(t, err)
		_, err = file.Write(pngData.Bytes())
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		return publicationHTTPRequest(t, client, http.MethodPost, publicationPath, actor, &body,
			map[string]string{"Content-Type": writer.FormDataContentType()})
	}
	countObjects := func(want int) {
		t.Helper()
		objects, err := inspector.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		require.NoError(t, err)
		require.Len(t, objects.Contents, want)
	}

	require.Equal(t, http.StatusUnauthorized, upload("", "Round trip").status)
	countObjects(0)
	firstResponse := upload("owner", "Round trip")
	first := decodePublicationHTTP(t, firstResponse)
	require.Positive(t, first.ID)
	require.Equal(t, actors["owner"], first.OwnerUserID)
	require.Equal(t, service.CreationPublicationPublished, first.Status)
	require.Equal(t, "image/png", first.MIME)
	require.EqualValues(t, pngData.Len(), first.Size)
	require.NotContains(t, string(firstResponse.body), "storage_key")
	require.NotContains(t, string(firstResponse.body), "storage_profile_id")
	require.NotContains(t, string(firstResponse.body), endpoint)
	stored, err := repo.GetByRequestID(ctx, actors["owner"], requestID)
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.ID)
	require.Equal(t, "backup", stored.StorageProfileID)
	require.True(t, strings.HasPrefix(stored.StorageKey, "roundtrip/creation-publications/"))
	head, err := inspector.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(stored.StorageKey)})
	require.NoError(t, err)
	require.Equal(t, s3types.ServerSideEncryptionAes256, head.ServerSideEncryption)
	require.EqualValues(t, pngData.Len(), aws.ToInt64(head.ContentLength))
	countObjects(1)

	replayed := decodePublicationHTTP(t, upload("owner", "Round trip"))
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, first.MediaURL, replayed.MediaURL)
	countObjects(1)
	var rowCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation_publications WHERE owner_user_id = $1 AND request_id = $2`, actors["owner"], requestID).Scan(&rowCount))
	require.Equal(t, 1, rowCount)
	require.Equal(t, http.StatusConflict, upload("owner", "Changed title").status)
	countObjects(1)

	mediaURL := server.URL + first.MediaURL
	public := publicationHTTPRequest(t, client, http.MethodGet, mediaURL, "", nil, nil)
	require.Equal(t, http.StatusOK, public.status, "%s", public.body)
	require.Equal(t, pngData.Bytes(), public.body)
	require.Equal(t, "image/png", public.header.Get("Content-Type"))
	require.Equal(t, "no-store", public.header.Get("Cache-Control"))
	require.Equal(t, "nosniff", public.header.Get("X-Content-Type-Options"))
	ranged := publicationHTTPRequest(t, client, http.MethodGet, mediaURL, "", nil, map[string]string{"Range": "bytes=0-15"})
	require.Equal(t, http.StatusPartialContent, ranged.status)
	require.Equal(t, pngData.Bytes()[:16], ranged.body)
	require.Equal(t, fmt.Sprintf("bytes 0-15/%d", pngData.Len()), ranged.header.Get("Content-Range"))
	unsigned := publicationHTTPRequest(t, client, http.MethodGet, endpoint+"/"+bucket+"/"+stored.StorageKey, "", nil, nil)
	require.Equal(t, http.StatusForbidden, unsigned.status, "the S3 bucket must remain private")

	statusURL := publicationPath + "/requests/" + requestID
	require.Equal(t, http.StatusUnauthorized, publicationHTTPRequest(t, client, http.MethodGet, statusURL, "", nil, nil).status)
	require.Equal(t, http.StatusNotFound, publicationHTTPRequest(t, client, http.MethodGet, statusURL, "other", nil, nil).status)
	require.Equal(t, first.ID, decodePublicationHTTP(t, publicationHTTPRequest(t, client, http.MethodGet, statusURL, "owner", nil, nil)).ID)
	deleteURL := fmt.Sprintf("%s/%d", publicationPath, first.ID)
	require.Equal(t, http.StatusUnauthorized, publicationHTTPRequest(t, client, http.MethodDelete, deleteURL, "", nil, nil).status)
	require.Equal(t, http.StatusNotFound, publicationHTTPRequest(t, client, http.MethodDelete, deleteURL, "other", nil, nil).status)
	countObjects(1)
	require.Equal(t, http.StatusOK, publicationHTTPRequest(t, client, http.MethodDelete, deleteURL, "owner", nil, nil).status)
	require.Equal(t, http.StatusNotFound, publicationHTTPRequest(t, client, http.MethodGet, mediaURL, "", nil, nil).status)
	tombstone := decodePublicationHTTP(t, publicationHTTPRequest(t, client, http.MethodGet, statusURL, "owner", nil, nil))
	require.NotNil(t, tombstone.WithdrawnAt)
	require.Empty(t, tombstone.MediaURL)
	countObjects(0)
	_, err = inspector.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(stored.StorageKey)})
	var missing *smithyhttp.ResponseError
	require.ErrorAs(t, err, &missing)
	require.Equal(t, http.StatusNotFound, missing.HTTPStatusCode())
	require.Equal(t, http.StatusConflict, upload("owner", "Round trip").status, "withdrawn request IDs must not republish")
	require.Equal(t, http.StatusOK, publicationHTTPRequest(t, client, http.MethodDelete, deleteURL, "owner", nil, nil).status)
	countObjects(0)
}
