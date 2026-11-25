package app

import (
	"encoding/json"
	"errors"
)

func (a *App) LocalComments(dramaID string, episode int) (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	return a.store.LocalComments(dramaID, episode)
}

func (a *App) SaveLocalComments(dramaID string, episode int, snapshot string) error {
	if a.store == nil {
		return ErrNoStore
	}
	if len(snapshot) > 256*1024 {
		return errors.New("本集评论数据过大")
	}
	var comments []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(snapshot), &comments); err != nil || comments == nil {
		return errors.New("本集评论数据无效")
	}
	for _, comment := range comments {
		if comment.ID == "" || comment.Text == "" {
			return errors.New("本集评论数据无效")
		}
	}
	if err := a.validateInteractionEpisode(dramaID, episode); err != nil {
		return err
	}
	return a.store.SaveLocalComments(dramaID, episode, snapshot)
}
