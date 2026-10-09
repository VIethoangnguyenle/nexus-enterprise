package domain

import "strings"

// publicEmailDomains are mailbox providers anyone can register an address
// with. An address at one of these says nothing about which company a person
// works for, so these domains are never used to place a user in a tenant —
// otherwise the first tenant to claim gmail.com would capture every Gmail user
// who signs up after it.
//
// This is the single list for every domain-based decision in the auth
// service: password signup auto-join, Google sign-in, and tenant domain claims.
var publicEmailDomains = map[string]struct{}{
	// Google
	"gmail.com": {}, "googlemail.com": {},
	// Microsoft
	"outlook.com": {}, "hotmail.com": {}, "live.com": {}, "msn.com": {},
	"outlook.com.vn": {}, "hotmail.co.uk": {}, "live.co.uk": {},
	// Yahoo / AOL
	"yahoo.com": {}, "yahoo.com.vn": {}, "yahoo.co.uk": {}, "ymail.com": {},
	"rocketmail.com": {}, "aol.com": {},
	// Apple
	"icloud.com": {}, "me.com": {}, "mac.com": {},
	// Proton
	"proton.me": {}, "protonmail.com": {}, "pm.me": {},
	// GMX / web.de / mail.com
	"gmx.com": {}, "gmx.net": {}, "gmx.de": {}, "web.de": {}, "mail.com": {},
	// Yandex / Mail.ru
	"yandex.com": {}, "yandex.ru": {}, "ya.ru": {}, "mail.ru": {},
	"bk.ru": {}, "inbox.ru": {}, "list.ru": {},
	// Others
	"zoho.com": {}, "zohomail.com": {}, "tutanota.com": {}, "tuta.io": {},
	"fastmail.com": {}, "hey.com": {}, "qq.com": {}, "163.com": {}, "126.com": {},
	"naver.com": {}, "daum.net": {},
}

// normalizeDomain lowercases a domain and strips surrounding space and a
// trailing root dot, so "ACME.com." and "acme.com" compare equal.
func normalizeDomain(d string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
}

// IsPublicEmailDomain reports whether d is a public mailbox provider whose
// addresses must never be used to auto-join or claim a tenant.
func IsPublicEmailDomain(d string) bool {
	_, ok := publicEmailDomains[normalizeDomain(d)]
	return ok
}
