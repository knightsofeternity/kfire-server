package store

// PresenceStatus computes a member's visible presence from whether they have an
// open game session, whether that session is visible to the viewer, and whether
// they hold a live WebSocket connection. An open session (any source, e.g. an
// Xbox console with no desktop client) shows in_game when visible; otherwise a
// live connection shows online; otherwise offline. A hidden open session does
// not reveal in_game and only yields online when WS-connected.
func PresenceStatus(hasOpenSession, sessionVisible, wsConnected bool) string {
	if hasOpenSession && sessionVisible {
		return "in_game"
	}
	if wsConnected {
		return "online"
	}
	return "offline"
}

// ApplyPresenceOverride forces "offline" when the member chose to hide their
// presence (invisible/offline), regardless of the computed status.
func ApplyPresenceOverride(chosen, computed string) string {
	if chosen == "invisible" || chosen == "offline" {
		return "offline"
	}
	return computed
}
