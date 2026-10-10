package store

import "strings"

// personName is the SQL for what a person is called in one workspace: their
// display name, else their login, and nothing at all when they are not a member
// of it. u is the users alias, tu the tenant_users alias joined on the
// workspace; because the name is read through that join, one workspace never
// reads a name another keeps for the same account.
func personName(u, tu string) string {
	return "(CASE WHEN " + tu + ".user_id IS NULL THEN '' ELSE COALESCE(NULLIF(" + u + ".display_name, ''), " + u + ".username) END)"
}

// personLogin is the member's login, or NULL for someone outside the workspace.
func personLogin(u, tu string) string {
	return "(CASE WHEN " + tu + ".user_id IS NULL THEN NULL ELSE " + u + ".username END)"
}

// memberJoin joins a user column to its membership of the row's workspace and
// to the account: `LEFT JOIN tenant_users tu ... LEFT JOIN users u ...`.
func memberJoin(userCol, wsCol, u, tu string) string {
	return " LEFT JOIN tenant_users " + tu + " ON " + tu + ".user_id = " + userCol + " AND " + tu + ".tenant_id = " + wsCol +
		" LEFT JOIN users " + u + " ON " + u + ".id = " + tu + ".user_id"
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// containsPattern turns user text into an ILIKE pattern that matches it
// literally anywhere (use with ESCAPE '\').
func containsPattern(s string) string {
	return "%" + likeEscaper.Replace(strings.TrimSpace(s)) + "%"
}
