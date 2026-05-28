package admin

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseImportArchiveRejectsAggregateJSONPayloadTooLarge(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entryPayload := archiveImportJSONFixture(archiveImportMaxJSONSize - 512)
	entryCount := archiveImportMaxSize/int64(len(entryPayload)) + 2
	for i := int64(0); i < entryCount; i++ {
		w, err := zw.Create(fmt.Sprintf("backup-%02d.json", i))
		require.NoError(t, err)
		_, err = w.Write(entryPayload)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	_, _, err := parseImportArchive(buf.Bytes())

	require.Error(t, err)
	require.Contains(t, err.Error(), "JSON 总大小")
}

func archiveImportJSONFixture(targetSize int64) []byte {
	const prefix = `{"type":"sub2api","version":"1.0","accounts":[],"padding":"`
	const suffix = `"}`
	paddingLen := int(targetSize) - len(prefix) - len(suffix)
	if paddingLen < 0 {
		paddingLen = 0
	}
	return []byte(prefix + strings.Repeat("a", paddingLen) + suffix)
}
