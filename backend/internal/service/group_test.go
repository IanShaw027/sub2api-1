//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGroup_GetImagePrice_1K 测试 1K 尺寸返回正确价格
func TestGroup_GetImagePrice_1K(t *testing.T) {
	price := 0.10
	group := &Group{
		ImagePrice1K: &price,
	}

	result := group.GetImagePrice("1K")
	require.NotNil(t, result)
	require.InDelta(t, 0.10, *result, 0.0001)
}

// TestGroup_GetImagePrice_2K 测试 2K 尺寸返回正确价格
func TestGroup_GetImagePrice_2K(t *testing.T) {
	price := 0.15
	group := &Group{
		ImagePrice2K: &price,
	}

	result := group.GetImagePrice("2K")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)
}

// TestGroup_GetImagePrice_4K 测试 4K 尺寸返回正确价格
func TestGroup_GetImagePrice_4K(t *testing.T) {
	price := 0.30
	group := &Group{
		ImagePrice4K: &price,
	}

	result := group.GetImagePrice("4K")
	require.NotNil(t, result)
	require.InDelta(t, 0.30, *result, 0.0001)
}

// TestGroup_GetImagePrice_UnknownSize 测试未知尺寸回退 2K
func TestGroup_GetImagePrice_UnknownSize(t *testing.T) {
	price2K := 0.15
	group := &Group{
		ImagePrice2K: &price2K,
	}

	// 未知尺寸 "3K" 应该回退到 2K
	result := group.GetImagePrice("3K")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)

	// 空字符串也回退到 2K
	result = group.GetImagePrice("")
	require.NotNil(t, result)
	require.InDelta(t, 0.15, *result, 0.0001)
}

// TestGroup_GetImagePrice_NilValues 测试未配置时返回 nil
func TestGroup_GetImagePrice_NilValues(t *testing.T) {
	group := &Group{
		// 所有 ImagePrice 字段都是 nil
	}

	require.Nil(t, group.GetImagePrice("1K"))
	require.Nil(t, group.GetImagePrice("2K"))
	require.Nil(t, group.GetImagePrice("4K"))
	require.Nil(t, group.GetImagePrice("unknown"))
}

// TestGroup_GetImagePrice_PartialConfig 测试部分配置
func TestGroup_GetImagePrice_PartialConfig(t *testing.T) {
	price1K := 0.10
	group := &Group{
		ImagePrice1K: &price1K,
		// ImagePrice2K 和 ImagePrice4K 未配置
	}

	result := group.GetImagePrice("1K")
	require.NotNil(t, result)
	require.InDelta(t, 0.10, *result, 0.0001)

	// 2K 和 4K 返回 nil
	require.Nil(t, group.GetImagePrice("2K"))
	require.Nil(t, group.GetImagePrice("4K"))
}

func TestGroup_GetImagePriceForRequestType_Images2APITiers(t *testing.T) {
	price1K := 0.11
	price2K := 0.22
	price4K := 0.44
	group := &Group{
		Images2APIPrice1K: &price1K,
		Images2APIPrice2K: &price2K,
		Images2APIPrice4K: &price4K,
	}

	result := group.GetImagePriceForRequestType("1K", RequestTypeImageWebBridge)
	require.NotNil(t, result)
	require.InDelta(t, 0.11, *result, 0.0001)

	result = group.GetImagePriceForRequestType("2K", RequestTypeImageWebBridge)
	require.NotNil(t, result)
	require.InDelta(t, 0.22, *result, 0.0001)

	result = group.GetImagePriceForRequestType("4K", RequestTypeImageWebBridge)
	require.NotNil(t, result)
	require.InDelta(t, 0.44, *result, 0.0001)
}

func TestGroup_GetImagePriceForRequestType_Images2APIUnknownSizeFallsBackTo2K(t *testing.T) {
	price2K := 0.22
	group := &Group{
		Images2APIPrice2K: &price2K,
	}

	result := group.GetImagePriceForRequestType("unknown", RequestTypeImageWebBridge)
	require.NotNil(t, result)
	require.InDelta(t, 0.22, *result, 0.0001)
}

func TestNormalizeGroupImageGenerationRoute_PreservesWeb2API(t *testing.T) {
	require.Equal(t, GroupImageGenerationRouteCodex, NormalizeGroupImageGenerationRoute(""))
	require.Equal(t, GroupImageGenerationRouteCodex, NormalizeGroupImageGenerationRoute("codex"))
	require.Equal(t, GroupImageGenerationRouteWeb2API, NormalizeGroupImageGenerationRoute("web2api"))
	require.Equal(t, GroupImageGenerationRouteWeb2API, (&Group{ImageGenerationRoute: "web2api"}).EffectiveImageGenerationRoute())
}

func TestNormalizeGroupVideoGenerationRoute_DefaultsToNative(t *testing.T) {
	require.Equal(t, "native", NormalizeGroupVideoGenerationRoute(""))
	require.Equal(t, "native", NormalizeGroupVideoGenerationRoute("native"))
	require.Equal(t, "native", (&Group{}).EffectiveVideoGenerationRoute())
}

func TestGroup_GetImagePriceConfigForRequestType_UsesImages2APIConfig(t *testing.T) {
	price1K := 0.11
	price2K := 0.22
	price4K := 0.44
	group := &Group{
		Images2APIPrice1K: &price1K,
		Images2APIPrice2K: &price2K,
		Images2APIPrice4K: &price4K,
	}

	config := group.GetImagePriceConfigForRequestType(RequestTypeImageWebBridge)
	require.NotNil(t, config)
	require.Equal(t, group.Images2APIPrice1K, config.Price1K)
	require.Equal(t, group.Images2APIPrice2K, config.Price2K)
	require.Equal(t, group.Images2APIPrice4K, config.Price4K)
}

func TestGroup_GetImagePriceConfigForRequestType_UsesStandardImagesConfig(t *testing.T) {
	price1K := 0.10
	price2K := 0.20
	price4K := 0.40
	group := &Group{
		ImagePrice1K: &price1K,
		ImagePrice2K: &price2K,
		ImagePrice4K: &price4K,
	}

	config := group.GetImagePriceConfigForRequestType(RequestTypeImage)
	require.NotNil(t, config)
	require.Equal(t, group.ImagePrice1K, config.Price1K)
	require.Equal(t, group.ImagePrice2K, config.Price2K)
	require.Equal(t, group.ImagePrice4K, config.Price4K)
}

func TestGroup_DisplayLabel_FallsBackToName(t *testing.T) {
	group := &Group{Name: "route-a", DisplayName: " "}
	require.Equal(t, "route-a", group.DisplayLabel())
}

func TestGroup_DisplayLabel_PrefersDisplayName(t *testing.T) {
	group := &Group{Name: "route-a", DisplayName: " Route A "}
	require.Equal(t, "Route A", group.DisplayLabel())
}

func TestGroup_CanBeSelectedByUser(t *testing.T) {
	group := &Group{Status: StatusActive, UserSelectable: true}
	require.True(t, group.CanBeSelectedByUser())

	group.UserSelectable = false
	require.False(t, group.CanBeSelectedByUser())

	group.Status = StatusDisabled
	group.UserSelectable = true
	require.False(t, group.CanBeSelectedByUser())
}
