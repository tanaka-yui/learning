package main

import (
	"embed"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

//go:embed model.conf policy.csv abac_model.conf
var policyFS embed.FS

// Resource は ABAC で使うリソース属性(所有者情報を持つ)
type Resource struct {
	ID    string
	Owner string
}

// newRBACEnforcer は model.conf + policy.csv から RBAC エンフォーサを生成する。
// PDP(Policy Decision Point)として機能する。
func newRBACEnforcer() (*casbin.Enforcer, error) {
	modelData, err := policyFS.ReadFile("model.conf")
	if err != nil {
		return nil, err
	}
	policyData, err := policyFS.ReadFile("policy.csv")
	if err != nil {
		return nil, err
	}

	m, err := model.NewModelFromString(string(modelData))
	if err != nil {
		return nil, err
	}

	return casbin.NewEnforcer(m, NewBytesAdapter(policyData))
}

// newABACEnforcer は abac_model.conf から ABAC エンフォーサを生成する。
// ポリシーは不要(マッチャー内で属性比較のみ)。
func newABACEnforcer() (*casbin.Enforcer, error) {
	modelData, err := policyFS.ReadFile("abac_model.conf")
	if err != nil {
		return nil, err
	}
	m, err := model.NewModelFromString(string(modelData))
	if err != nil {
		return nil, err
	}
	// ABAC はポリシー行が不要なので空アダプタ
	return casbin.NewEnforcer(m, NewBytesAdapter([]byte{}))
}
