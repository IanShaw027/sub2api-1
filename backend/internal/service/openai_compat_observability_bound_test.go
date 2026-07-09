package service

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRecordBoundedModelMetric_CapsDistinctKeys 外部可控 model 名超过上限后并入溢出桶，
// map 基数封顶不再无界增长。
func TestRecordBoundedModelMetric_CapsDistinctKeys(t *testing.T) {
	counts := make(map[string]int64)

	// 灌入远超上限数量的不同 model 名。
	for i := 0; i < maxOpenAICompatModelMetricKeys+500; i++ {
		recordBoundedModelMetric(counts, "model-"+strconv.Itoa(i))
	}

	// 常规 key 数封顶为上限，额外一个溢出桶。
	require.LessOrEqual(t, len(counts), maxOpenAICompatModelMetricKeys+1)
	require.Contains(t, counts, openAICompatModelMetricOverflowKey)
	require.Equal(t, int64(500), counts[openAICompatModelMetricOverflowKey])

	// 已存在的 key 仍能继续自增，不受上限影响。
	existing := "model-0"
	before := counts[existing]
	recordBoundedModelMetric(counts, existing)
	require.Equal(t, before+1, counts[existing])
}
