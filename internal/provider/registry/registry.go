// Package registry 是 Provider 适配层的统一入口：按名称取得 Provider。
// 四个适配器与架构图一致：mock、licensed、hongguo-web、hongguo-app。
// 两个红果适配器在取得官方授权前恒为禁用桩（不发网络请求）。
package registry

import (
	"fmt"
	"sort"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/authorized"
	"jindi-drama/internal/provider/hongguoweb"
	"jindi-drama/internal/provider/licensed"
	"jindi-drama/internal/provider/mock"
)

const (
	Mock       = "mock"
	Licensed   = "licensed"
	HongguoWeb = "hongguo-web"
	HongguoApp = "hongguo-app"
)

// Default 是默认且唯一可用于演示的 Provider。
const Default = Mock

// Options 为需要配置的适配器提供参数。
type Options struct{ LicensedMediaDir string }

func Names() []string {
	n := []string{Mock, Licensed, HongguoWeb, HongguoApp}
	sort.Strings(n)
	return n
}

// New 按名称创建 Provider；未知名称返回错误（调用方应回退到 Default）。
func New(name string, o Options) (provider.VideoProvider, error) {
	switch name {
	case "", Mock:
		return mock.New(), nil
	case Licensed:
		return licensed.New(o.LicensedMediaDir)
	case HongguoWeb:
		return hongguoweb.New(), nil
	case HongguoApp:
		return authorized.New(), nil
	}
	return nil, fmt.Errorf("registry: 未知 Provider %q", name)
}

// NewOrDefault 在名称无效时回退到 Mock，保证应用始终可用。
func NewOrDefault(name string, o Options) provider.VideoProvider {
	p, err := New(name, o)
	if err != nil {
		return mock.New()
	}
	return p
}
