package service

import "strings"

const (
	openFGATypeUser      = "user"
	openFGATypeGroup     = "group"
	openFGATypeFolder    = "folder"
	openFGATypeFile      = "file"
	openFGATypeShareLink = "share_link"
)

func openFGAUser(publicID string) string {
	return openFGAObject(openFGATypeUser, publicID)
}

func openFGAGroup(publicID string) string {
	return openFGAObject(openFGATypeGroup, publicID)
}

func openFGAFolder(publicID string) string {
	return openFGAObject(openFGATypeFolder, publicID)
}

func openFGAFile(publicID string) string {
	return openFGAObject(openFGATypeFile, publicID)
}

func openFGAShareLink(publicID string) string {
	return openFGAObject(openFGATypeShareLink, publicID)
}

func openFGAGroupMember(publicID string) string {
	return openFGAGroup(publicID) + "#member"
}

func openFGAObject(objectType, publicID string) string {
	return strings.TrimSpace(objectType) + ":" + strings.TrimSpace(publicID)
}

func stripOpenFGAObjectPrefix(value string) string {
	if _, publicID, ok := strings.Cut(strings.TrimSpace(value), ":"); ok {
		return publicID
	}
	return strings.TrimSpace(value)
}
