package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
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
