//go:build unit

package repository

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

func TestBuildMediaPutObjectInputPrivateSSE(t *testing.T) {
	in := buildMediaPutObjectInput("bucket", "media/invoice/a.pdf", "application/pdf", []byte("pdf"))
	require.Equal(t, types.ServerSideEncryptionAes256, in.ServerSideEncryption)
	require.Empty(t, string(in.ACL))
	require.NotEqual(t, types.ObjectCannedACLPublicRead, in.ACL)
	require.NotNil(t, in.Bucket)
	require.Equal(t, "bucket", *in.Bucket)
	require.Equal(t, "media/invoice/a.pdf", *in.Key)
}
