package app

import (
	"fmt"
	"strconv"

	"jindi-drama/internal/provider"
)

func (a *App) LocalEpisodeLiked(dramaID string, episode int) (bool, error) {
	a.awaitStartup()
	if a.store == nil {
		return false, ErrNoStore
	}
	if err := a.validateDemoEpisodeLike(dramaID, episode); err != nil {
		return false, err
	}
	value, exists, err := a.store.GetSetting(episodeLikeKey(dramaID, episode))
	if err != nil || !exists {
		return false, err
	}
	return strconv.ParseBool(value)
}

func (a *App) SetLocalEpisodeLiked(dramaID string, episode int, liked bool) error {
	a.awaitStartup()
	if a.store == nil {
		return ErrNoStore
	}
	if err := a.validateDemoEpisodeLike(dramaID, episode); err != nil {
		return err
	}
	return a.store.SetSetting(episodeLikeKey(dramaID, episode), strconv.FormatBool(liked))
}

func episodeLikeKey(dramaID string, episode int) string {
	return fmt.Sprintf("episode_like/%s/%d", dramaID, episode)
}

func (a *App) validateDemoEpisodeLike(dramaID string, episode int) error {
	if a.providerForDrama(dramaID).Name() != "mock" {
		return provider.ErrEpisodeUnavailable
	}
	return a.validateInteractionEpisode(dramaID, episode)
}
