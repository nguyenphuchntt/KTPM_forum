package request

// RequestUploadRequest asks for a pre-signed URL to PUT an image to. The
// filename and size are hints only: they let the server reject obviously wrong
// uploads before handing out a URL, and the content is re-checked on confirm.
type RequestUploadRequest struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// ConfirmUploadRequest tells the server an upload has finished, so it can
// validate the object and promote it out of quarantine.
type ConfirmUploadRequest struct {
	ObjectKey string `json:"object_key"`
}
