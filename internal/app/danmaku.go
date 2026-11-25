package app

import (
	"errors"
	"strings"
	"unicode/utf16"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/storage/sqlite"
)

func (a *App) LocalDanmaku(dramaID string, episode int) ([]sqlite.LocalDanmaku, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.LocalDanmaku(dramaID, episode)
}

func (a *App) SendLocalDanmaku(dramaID string, episode int, offsetMS int64, text string) (sqlite.LocalDanmaku, error) {
	if a.store == nil {
		return sqlite.LocalDanmaku{}, ErrNoStore
	}
	text = strings.TrimSpace(text)
	if len(text) == 0 || len(text) > 160 || len(utf16.Encode([]rune(text))) > 40 || offsetMS < 0 {
		return sqlite.LocalDanmaku{}, errors.New("弹幕内容或时间无效")
	}
	if err := a.validateInteractionEpisode(dramaID, episode); err != nil {
		return sqlite.LocalDanmaku{}, err
	}
	return a.store.AddLocalDanmaku(dramaID, episode, offsetMS, text)
}

func (a *App) validateInteractionEpisode(dramaID string, episode int) error {
	drama, err := a.Drama(dramaID)
	if err != nil {
		return err
	}
	if episode < 1 || episode > drama.UpdatedTo {
		return provider.ErrEpisodeUnavailable
	}
	return nil
}
