package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// mailpitHost は送信先SMTPホストを返す(環境変数 MAILPIT_HOST、既定 localhost:1025)。
func mailpitHost() string {
	if h := os.Getenv("MAILPIT_HOST"); h != "" {
		return h
	}
	return "localhost:1025"
}

// sendMagicLinkMail はマジックリンクをSMTPで送信する。
// Mailpit などの開発用SMTPサーバ(認証なし)を想定する。
// 送信に失敗してもトークン発行自体は成立させたいので、エラーは呼び出し側で
// ベストエフォート扱いにする(致命的にしない)。
func sendMagicLinkMail(host, to, link string) error {
	from := "no-reply@mfa-demo.local"
	msg := buildMail(from, to, link)
	// 開発用SMTP(Mailpit)は認証不要のため auth は nil。
	return smtp.SendMail(host, nil, from, []string{to}, []byte(msg))
}

// buildMail はRFC 5322 形式の簡易メール本文を組み立てる。
func buildMail(from, to, link string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", "ログイン用マジックリンク")
	fmt.Fprint(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprint(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprint(&b, "\r\n")
	fmt.Fprint(&b, "以下のリンクをクリックするとログインできます(1回のみ有効)。\r\n\r\n")
	fmt.Fprintf(&b, "%s\r\n", link)
	return b.String()
}
