package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"

	"jindi-drama/internal/storage/sqlite"
)

const maxLocalCommentSnapshot = 256 * 1024

var errInvalidLocalComments = errors.New("本集评论数据无效")

type localCommentFields map[string]json.RawMessage

func (fields *localCommentFields) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return errInvalidLocalComments
	}
	record := make(localCommentFields)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok {
			return errInvalidLocalComments
		}
		if _, duplicate := record[name]; duplicate {
			return errInvalidLocalComments
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return errInvalidLocalComments
		}
		record[name] = value
	}
	// Unmarshal supplies one complete JSON value to this method.
	*fields = record
	return nil
}

func (a *App) LocalComments(dramaID string, episode int) (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	snapshot, found, err := a.store.LocalComments(dramaID, episode)
	if err != nil || !found {
		return "", err
	}
	if err := validateLocalCommentSnapshot(snapshot); err != nil {
		return "", err
	}
	return snapshot, nil
}

func (a *App) SaveLocalComments(dramaID string, episode int, snapshot string) error {
	if a.store == nil {
		return ErrNoStore
	}
	if err := validateLocalCommentSnapshot(snapshot); err != nil {
		return err
	}
	if err := a.validateInteractionEpisode(dramaID, episode); err != nil {
		return err
	}
	return a.store.SaveLocalComments(dramaID, episode, snapshot)
}

type localCommentState struct {
	sqlite.LocalCommentState
	Profile       string `json:"profile"`
	LegacyProfile string `json:"legacyProfile"`
}

// LocalCommentsState identifies the open database without disclosing its path.
func (a *App) LocalCommentsState(dramaID string, episode int) (localCommentState, error) {
	a.awaitStartup()
	if a.store == nil {
		return localCommentState{}, ErrNoStore
	}
	profile, err := a.store.DataProfile()
	if err != nil {
		return localCommentState{}, err
	}
	current, err := a.store.LocalCommentsState(dramaID, episode)
	if err != nil {
		return localCommentState{}, err
	}
	if current.Revision != "" {
		if err := validateLocalCommentSnapshot(current.Snapshot); err != nil {
			return localCommentState{}, err
		}
	}
	return localCommentState{
		LocalCommentState: current,
		Profile:           profile,
		LegacyProfile:     a.store.CommentProfile(),
	}, nil
}

func (a *App) CompareAndSaveLocalComments(dramaID string, episode int, expected, snapshot, writeID string) (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	if (expected != "" && !validLocalWriteID(expected, 64)) || !validLocalWriteID(writeID, 32) {
		return "", errInvalidLocalComments
	}
	if err := validateLocalCommentSnapshot(snapshot); err != nil {
		return "", err
	}
	if err := a.validateInteractionEpisode(dramaID, episode); err != nil {
		return "", err
	}
	return a.store.CompareAndSaveLocalComments(dramaID, episode, expected, snapshot, writeID)
}

func validLocalWriteID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func validateLocalCommentSnapshot(snapshot string) error {
	if len(snapshot) > maxLocalCommentSnapshot {
		return errors.New("本集评论数据过大")
	}
	var comments []localCommentFields
	if err := json.Unmarshal([]byte(snapshot), &comments); err != nil || comments == nil {
		return errInvalidLocalComments
	}
	commentIDs := make(map[string]struct{}, len(comments))
	for _, comment := range comments {
		if !recordLocalComment(comment, commentIDs) ||
			!decodeOptionalCommentField(comment, "ts", new(float64)) ||
			!decodeOptionalCommentField(comment, "byAuthor", new(bool)) {
			return errInvalidLocalComments
		}
		var replies []localCommentFields
		if raw, present := comment["replies"]; present {
			if err := json.Unmarshal(raw, &replies); err != nil || replies == nil {
				return errInvalidLocalComments
			}
		}
		count := float64(len(replies))
		if !decodeOptionalCommentField(comment, "replyTotal", &count) ||
			count < float64(len(replies)) || count > 1<<53-1 || math.Trunc(count) != count {
			return errInvalidLocalComments
		}
		replyIDs := make(map[string]struct{}, len(replies))
		for _, reply := range replies {
			if !recordLocalComment(reply, replyIDs) {
				return errInvalidLocalComments
			}
		}
	}
	return nil
}

func recordLocalComment(entry localCommentFields, seen map[string]struct{}) bool {
	var id, text string
	if json.Unmarshal(entry["id"], &id) != nil || json.Unmarshal(entry["text"], &text) != nil {
		return false
	}
	if _, duplicate := seen[id]; duplicate || id == "" || text == "" {
		return false
	}
	if !decodeOptionalCommentField(entry, "user", new(string)) ||
		!decodeOptionalCommentField(entry, "level", new(float64)) ||
		!decodeOptionalCommentField(entry, "likes", new(float64)) ||
		!decodeOptionalCommentField(entry, "liked", new(bool)) ||
		!decodeOptionalCommentField(entry, "ago", new(string)) ||
		!decodeOptionalCommentField(entry, "mine", new(bool)) {
		return false
	}
	seen[id] = struct{}{}
	return true
}

func decodeOptionalCommentField(entry localCommentFields, name string, into any) bool {
	raw, present := entry[name]
	return !present || (!bytes.Equal(raw, []byte("null")) && json.Unmarshal(raw, into) == nil)
}
