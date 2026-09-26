package usecase

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"forum/server/cloud"
	"forum/server/config"
	"forum/server/dto/request"
	"forum/server/dto/response"
	"forum/server/logger"
	"forum/server/model"
	mediaRepository "forum/server/repository/postgresql/media"
	"forum/server/validation"
	"forum/server/validation/filters"

	"github.com/google/uuid"
)

// UploadURLTTL is how long the client has to finish the PUT. It is deliberately
// short: the URL is a bearer credential for one object.
const UploadURLTTL = 5 * time.Minute

// objectKeyPrefix namespaces every uploaded object, and embeds the uploader so
// confirm can prove the object belongs to the caller.
const objectKeyPrefix = "images"

var (
	// ErrUploadNotFound is returned both for an unknown object and for one that
	// belongs to another account. The two cases share a message on purpose: a
	// distinguishable response would turn the endpoint into an oracle for other
	// people's object keys.
	ErrUploadNotFound     = NewAppError(http.StatusNotFound, CodeNotFound, "uploaded object not found")
	ErrUploadTooLarge     = NewAppError(http.StatusBadRequest, CodeValidationError, "file exceeds the 5MB limit")
	ErrUploadInvalidImage = NewAppError(http.StatusBadRequest, CodeValidationError, "uploaded file is not a valid image")
	ErrUploadTypeMismatch = NewAppError(http.StatusBadRequest, CodeValidationError, "file content does not match its extension")
	ErrMediaNotFound      = NewAppError(http.StatusNotFound, CodeNotFound, "image not found")
	ErrStorageUnavailable = NewAppError(http.StatusServiceUnavailable, CodeInternal, "object storage is unavailable")
	ErrUploadFailed       = NewAppError(http.StatusInternalServerError, CodeInternal, "upload could not be completed")
)

// UploadUsecase owns the valet-key upload flow: hand out a pre-signed PUT,
// then validate and promote whatever arrived at that key. The bytes never pass
// through this process, so the only trustworthy facts are the ones re-read from
// storage here.
type UploadUsecase struct {
	db      *sql.DB
	storage cloud.Storage
	cfg     *config.MinIOConfig
	auth    *AuthUsecase
}

func NewUploadUsecase(db *sql.DB, storage cloud.Storage, cfg *config.MinIOConfig, auth *AuthUsecase) *UploadUsecase {
	return &UploadUsecase{db: db, storage: storage, cfg: cfg, auth: auth}
}

// RequestUploadURL authenticates the caller, checks the claimed metadata and
// signs a PUT for a fresh key in the quarantine bucket. Nothing is written yet,
// so an abandoned request costs nothing.
func (uc *UploadUsecase) RequestUploadURL(r *http.Request, req request.RequestUploadRequest) (*response.UploadTicketResponse, error) {
	session, err := uc.auth.RequireSession(r)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(strings.TrimSpace(req.Filename))
	objectKey := fmt.Sprintf("%s/%s/%d%s", objectKeyPrefix, session.UserID, time.Now().UnixNano(), ext)

	uploadURL, err := uc.storage.PresignPut(r.Context(), uc.cfg.QuarantineBucket, objectKey, UploadURLTTL)
	if err != nil {
		log := logger.WithRequest(r, session.UserID)
		log.Error().Err(err).Str("object_key", objectKey).Msg("Failed to sign upload URL")
		return nil, ErrStorageUnavailable
	}

	log := logger.WithRequest(r, session.UserID)
	log.Info().Str("object_key", objectKey).Msg("Issued upload URL")

	return &response.UploadTicketResponse{
		UploadURL: uploadURL,
		ObjectKey: objectKey,
		ExpiresIn: int(UploadURLTTL.Seconds()),
	}, nil
}

// ConfirmUpload validates the object the client uploaded and, if it is a real
// image of an allowed type, promotes it to the public bucket and records it.
//
// Running this synchronously is what lets the response carry a URL that is
// already live, so the client never has to poll for the image to appear.
func (uc *UploadUsecase) ConfirmUpload(r *http.Request, req request.ConfirmUploadRequest) (*response.MediaResponse, error) {
	session, err := uc.auth.RequireSession(r)
	if err != nil {
		return nil, err
	}

	objectKey := strings.TrimSpace(req.ObjectKey)
	if !ownsObjectKey(session.UserID, objectKey) {
		return nil, ErrUploadNotFound
	}

	ctx := r.Context()
	log := logger.WithRequest(r, session.UserID)

	info, err := uc.storage.StatObject(ctx, uc.cfg.QuarantineBucket, objectKey)
	if err != nil {
		if errors.Is(err, cloud.ErrObjectNotFound) {
			return nil, ErrUploadNotFound
		}
		log.Error().Err(err).Str("object_key", objectKey).Msg("Failed to stat uploaded object")
		return nil, ErrStorageUnavailable
	}

	// The client's declared size is advisory; this is the real one.
	if info.Size <= 0 || info.Size > model.MaxMediaSize {
		uc.discardQuarantineObject(r, session.UserID, objectKey)
		return nil, ErrUploadTooLarge
	}

	data, err := uc.readObject(r, objectKey)
	if err != nil {
		uc.discardQuarantineObject(r, session.UserID, objectKey)
		if errors.Is(err, errObjectTooLarge) {
			return nil, ErrUploadTooLarge
		}
		log.Error().Err(err).Str("object_key", objectKey).Msg("Failed to read uploaded object")
		return nil, ErrStorageUnavailable
	}

	detectedType, err := validateImageContent(objectKey, data)
	if err != nil {
		uc.discardQuarantineObject(r, session.UserID, objectKey)
		log.Warn().Str("object_key", objectKey).Err(err).Msg("Rejected uploaded object")
		return nil, err
	}

	mimeType := model.ImageMIMEType(detectedType)

	// Promote first, then record: an object that is public but unrecorded is
	// harmless (nobody holds its id), whereas a row pointing at a missing
	// object would render as a broken image.
	if err := uc.storage.Copy(ctx, uc.cfg.QuarantineBucket, objectKey, uc.cfg.MediaBucket, objectKey); err != nil {
		log.Error().Err(err).Str("object_key", objectKey).Msg("Failed to promote uploaded object")
		return nil, ErrStorageUnavailable
	}

	publicURL := uc.storage.PublicURL(uc.cfg.MediaBucket, objectKey)

	mediaID, err := mediaRepository.StoreMedia(uc.db, session.UserID, objectKey, publicURL, mimeType, int64(len(data)))
	if err != nil {
		// Roll the promotion back so a failed confirm does not leak a public
		// object with no row pointing at it.
		if removeErr := uc.storage.Remove(ctx, uc.cfg.MediaBucket, objectKey); removeErr != nil {
			log.Error().Err(removeErr).Str("object_key", objectKey).Msg("Failed to roll back promoted object")
		}
		log.Error().Err(err).Str("object_key", objectKey).Msg("Failed to record media")
		return nil, ErrUploadFailed
	}

	// The quarantine copy has served its purpose. Failing to delete it must not
	// fail the upload — the media row is already valid.
	if err := uc.storage.Remove(ctx, uc.cfg.QuarantineBucket, objectKey); err != nil {
		log.Warn().Err(err).Str("object_key", objectKey).Msg("Failed to clean up quarantine object")
	}

	log.Info().Int64("media_id", int64(mediaID)).Str("object_key", objectKey).Msg("Image uploaded")

	return &response.MediaResponse{
		MediaID:   int64(mediaID),
		PublicURL: publicURL,
		MIMEType:  mimeType,
		Size:      int64(len(data)),
	}, nil
}

// ResolveMediaURL returns the stored public URL of a media row, for the route
// that renders <img src>.
func (uc *UploadUsecase) ResolveMediaURL(id model.MediaID) (string, error) {
	m, err := mediaRepository.LoadMedia(uc.db, id)
	if err != nil {
		if errors.Is(err, mediaRepository.ErrMediaNotFound) {
			return "", ErrMediaNotFound
		}
		return "", ErrUploadFailed
	}
	return m.PublicURL, nil
}

// errObjectTooLarge marks an object whose body exceeds the limit even though its
// reported size did not.
var errObjectTooLarge = errors.New("object too large")

// readObject pulls the quarantine object into memory. The read is capped at one
// byte over the limit, so neither a lying Content-Length nor a missing one can
// exhaust memory.
func (uc *UploadUsecase) readObject(r *http.Request, objectKey string) ([]byte, error) {
	body, err := uc.storage.GetObject(r.Context(), uc.cfg.QuarantineBucket, objectKey)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, model.MaxMediaSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > model.MaxMediaSize {
		return nil, errObjectTooLarge
	}
	return data, nil
}

// discardQuarantineObject best-effort deletes a rejected upload, so a client
// cannot fill the quarantine bucket by repeatedly posting garbage.
func (uc *UploadUsecase) discardQuarantineObject(r *http.Request, userID uuid.UUID, objectKey string) {
	if err := uc.storage.Remove(r.Context(), uc.cfg.QuarantineBucket, objectKey); err != nil {
		log := logger.WithRequest(r, userID)
		log.Warn().Err(err).Str("object_key", objectKey).Msg("Failed to discard rejected object")
	}
}

// validateImageContent runs the shared Pipes-and-Filters validation over the
// downloaded bytes and returns the sniffed image type. The filename extension
// must agree with that type, so a PNG renamed to .jpg is rejected rather than
// stored under a misleading key.
func validateImageContent(objectKey string, data []byte) (string, error) {
	ctx := &validation.ValidationContext{
		Reader:   bytes.NewReader(data),
		Filename: filepath.Base(objectKey),
		Size:     int64(len(data)),
		Metadata: make(map[string]interface{}),
	}

	if err := validation.NewPipeline(filters.NewMagicBytesFilter()).Execute(ctx); err != nil {
		return "", ErrUploadInvalidImage
	}

	detectedType, _ := ctx.Metadata["detected_type"].(string)
	if detectedType == "" {
		return "", ErrUploadInvalidImage
	}

	// WebP has no stdlib decoder, so its container is verified instead of its
	// pixels; the formats with decoders must survive a real decode.
	integrity := validation.NewPipeline(&filters.WebPFilter{})
	if detectedType != "webp" {
		integrity = validation.NewPipeline(&filters.ImageIntegrityFilter{})
	}
	if err := integrity.Execute(ctx); err != nil {
		return "", ErrUploadInvalidImage
	}

	if !model.ExtensionMatchesType(filepath.Ext(objectKey), detectedType) {
		return "", ErrUploadTypeMismatch
	}

	return detectedType, nil
}

// ownsObjectKey reports whether the key is one this account could have been
// issued. Keys are generated server-side, so anything else — another user's
// key, an absolute path, a traversal attempt — is treated as not found.
func ownsObjectKey(userID uuid.UUID, objectKey string) bool {
	prefix := objectKeyPrefix + "/" + userID.String() + "/"
	if !strings.HasPrefix(objectKey, prefix) {
		return false
	}

	remainder := strings.TrimPrefix(objectKey, prefix)
	if remainder == "" || remainder == "." || remainder == ".." {
		return false
	}

	for _, r := range remainder {
		isDigit := r >= '0' && r <= '9'
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		if !isDigit && !isLower && !isUpper && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
