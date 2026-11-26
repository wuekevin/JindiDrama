package app

import (
	"errors"
	"strings"
	"unicode/utf16"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/applecms"
	"jindi-drama/internal/storage/sqlite"
)

func (a *App) LocalDanmaku(dramaID string, episode int) ([]sqlite.LocalDanmaku, error) {
	if a.store == nil {
		return nil, ErrNoStore
	}
	return a.store.LocalDanmaku(dramaID, episode)
}

func (a *App) SendLocalDanmaku(dramaID string, episode int, offsetMS int64, text string) (sqlite.LocalDanmaku, error) {
	return a.sendLocalDanmaku(dramaID, episode, offsetMS, text, "")
}

func (a *App) SendLocalDanmakuOnce(dramaID string, episode int, offsetMS int64, text, requestID string) (sqlite.LocalDanmaku, error) {
	if !validLocalWriteID(requestID, 32) {
		return sqlite.LocalDanmaku{}, errors.New("弹幕请求标识无效")
	}
	return a.sendLocalDanmaku(dramaID, episode, offsetMS, text, requestID)
}

func (a *App) sendLocalDanmaku(dramaID string, episode int, offsetMS int64, text, requestID string) (sqlite.LocalDanmaku, error) {
	if a.store == nil {
		return sqlite.LocalDanmaku{}, ErrNoStore
	}
	text = strings.TrimSpace(text)
	if len(text) == 0 || len(text) > 160 || len(utf16.Encode([]rune(text))) > 40 || offsetMS < 0 {
		return sqlite.LocalDanmaku{}, errors.New("弹幕内容或时间无效")
	}
	if err := a.validateLocalInteractionEpisode(dramaID, episode); err != nil {
		return sqlite.LocalDanmaku{}, err
	}
	if requestID != "" {
		return a.store.AddLocalDanmakuOnce(dramaID, episode, offsetMS, text, requestID)
	}
	return a.store.AddLocalDanmaku(dramaID, episode, offsetMS, text)
}

func (a *App) validateLocalInteractionEpisode(dramaID string, episode int) error {
	if !strings.HasPrefix(dramaID, "apple-cms:") {
		return a.validateInteractionEpisode(dramaID, episode)
	}
	cms, ok := a.p.(*applecms.Provider)
	if !ok || !cms.OwnsDramaID(dramaID) {
		return provider.ErrNotFound
	}
	if episode < 1 || episode > applecms.MaxEpisodes {
		return provider.ErrEpisodeUnavailable
	}
	// A verified catalog page exposes the editor. Saving interactions only uses
	// local identity and remains available during upstream outages.
	return nil
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
