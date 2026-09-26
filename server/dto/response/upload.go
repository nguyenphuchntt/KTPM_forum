package response

// UploadTicketResponse is the pre-signed PUT the client uploads to. There is no
// public URL here on purpose: the image only becomes public after
// /api/upload/confirm has validated it.
type UploadTicketResponse struct {
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
	ExpiresIn int    `json:"expires_in"`
}

// MediaResponse describes a validated, promoted image.
type MediaResponse struct {
	MediaID   int64  `json:"media_id"`
	PublicURL string `json:"public_url"`
	MIMEType  string `json:"mime_type"`
	Size      int64  `json:"size"`
}
