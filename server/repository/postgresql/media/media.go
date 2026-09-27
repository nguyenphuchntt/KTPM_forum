package media

import (
	"database/sql"
	"errors"
	"fmt"

	"forum/server/database"
	"forum/server/model"

	"github.com/google/uuid"
)

// ErrMediaNotFound is returned when the id does not exist, or exists but
// belongs to another account.
var ErrMediaNotFound = errors.New("media not found")

// StoreMedia records an object that has already been validated and promoted to
// the public bucket, and returns its new id.
func StoreMedia(db *sql.DB, userID uuid.UUID, objectKey, publicURL, mimeType string, size int64) (model.MediaID, error) {
	query := `INSERT INTO medias (user_id, object_key, public_url, mime_type, size) VALUES (?,?,?,?,?) RETURNING id`

	var id int64
	row, recordError := database.QueryRowWithMetricsAndError(db, "insert_media", query,
		userID, objectKey, publicURL, mimeType, size)
	err := row.Scan(&id)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("insert media: %w", err)
	}
	return model.MediaID(id), nil
}

// LoadMedia loads a media row by id, whoever owns it. Used when serving an
// image, since reads are public.
func LoadMedia(db *sql.DB, id model.MediaID) (model.Media, error) {
	query := `SELECT id, object_key, public_url, mime_type, size, created_at, updated_at
	          FROM medias WHERE id = ?`
	return scanMedia(db, "select_media", query, int64(id))
}

// LoadMediaForOwner is LoadMedia narrowed to one account. It returns
// ErrMediaNotFound when the row exists but belongs to somebody else, so a
// caller cannot tell the two cases apart and use the endpoint to probe for
// other people's ids.
func LoadMediaForOwner(db *sql.DB, id model.MediaID, userID uuid.UUID) (model.Media, error) {
	query := `SELECT id, object_key, public_url, mime_type, size, created_at, updated_at
	          FROM medias WHERE id = ? AND user_id = ?`
	return scanMedia(db, "select_media_for_owner", query, int64(id), userID)
}

func scanMedia(db *sql.DB, op, query string, args ...interface{}) (model.Media, error) {
	var (
		m  model.Media
		id int64
	)
	row, recordError := database.QueryRowWithMetricsAndError(db, op, query, args...)
	err := row.Scan(&id, &m.ObjectKey, &m.PublicURL, &m.MIMEType, &m.Size, &m.CreatedAt, &m.UpdatedAt)
	recordError(err)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Media{}, ErrMediaNotFound
	}
	if err != nil {
		return model.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	m.ID = model.MediaID(id)
	return m, nil
}
