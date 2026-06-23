package main

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// BytesAdapter は []byte のポリシーをインメモリで提供するアダプタ。
// casbin v2 の file-adapter がファイルパスを必要とするため独自実装する。
type BytesAdapter struct {
	data []byte
}

// NewBytesAdapter は CSV バイト列からアダプタを生成する。
func NewBytesAdapter(data []byte) *BytesAdapter {
	return &BytesAdapter{data: data}
}

// LoadPolicy は CSV 行を読んでモデルにポリシーを追加する。
func (a *BytesAdapter) LoadPolicy(m model.Model) error {
	scanner := bufio.NewScanner(bytes.NewReader(a.data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		persist.LoadPolicyLine(line, m)
	}
	return scanner.Err()
}

// SavePolicy は今回は読み取り専用なので実装しない(学習デモ)。
func (a *BytesAdapter) SavePolicy(_ model.Model) error { return nil }

// AddPolicy は動的追加(今回は未使用)。
func (a *BytesAdapter) AddPolicy(_ string, _ string, _ []string) error { return nil }

// RemovePolicy は動的削除(今回は未使用)。
func (a *BytesAdapter) RemovePolicy(_ string, _ string, _ []string) error { return nil }

// RemoveFilteredPolicy はフィルタ削除(今回は未使用)。
func (a *BytesAdapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return nil
}
