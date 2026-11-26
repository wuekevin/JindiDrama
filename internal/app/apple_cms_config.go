package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/applecms"
)

const (
	appleCMSURLKey       = "apple_cms_url"
	maxAppleCMSURLLength = 4096
)

var (
	errAppleCMSConfig     = errors.New("苹果 CMS 配置无效，继续使用演示目录")
	errAppleCMSUserURL    = errors.New("请输入无需凭据或查询参数的公网 HTTPS 地址")
	errAppleCMSRead       = errors.New("无法读取已保存的苹果 CMS 片源")
	errAppleCMSStoredURL  = errors.New("已保存的苹果 CMS 片源地址无效")
	errAppleCMSManaged    = errors.New("苹果 CMS 片源由运营方配置")
	errAppleCMSConnection = errors.New("无法连接片源或读取苹果 CMS 目录")
	errAppleCMSWrite      = errors.New("无法保存苹果 CMS 片源设置")
)

type AppleCMSConfiguration struct {
	Active          bool   `json:"active"`
	SavedURL        string `json:"savedURL"`
	Managed         bool   `json:"managed"`
	CanConfigure    bool   `json:"canConfigure"`
	RestartRequired bool   `json:"restartRequired"`
	ReadError       string `json:"readError,omitempty"`
}

// PrepareAppleCMS selects the startup provider before the desktop registers its bindings.
func PrepareAppleCMS(a *App, managedURL string) error {
	return prepareAppleCMS(a, managedURL, newAppleCMSClient())
}

func prepareAppleCMS(a *App, managedURL string, client *http.Client) error {
	a.appleCMSManaged = managedURL != ""
	if a.appleCMSManaged {
		a.appleCMSStartupErr = configureAppleCMS(a, managedURL, client)
		return a.appleCMSStartupErr
	}
	if a.store == nil {
		if err := a.openStore(""); err != nil {
			a.appleCMSStartupErr = errAppleCMSRead
			return errAppleCMSRead
		}
	}
	saved, err := a.savedAppleCMSURL()
	if err != nil || saved == "" {
		a.appleCMSStartupErr = err
		return err
	}
	if err := configureAppleCMS(a, saved, client); err != nil {
		a.appleCMSStartupErr = err
		return err
	}
	a.appleCMSStartupURL = saved
	return nil
}

func newAppleCMSClient() *http.Client {
	client := newCatalogMediaClient()
	client.Timeout = 12 * time.Second
	return client
}

// ConfigureAppleCMS configures an explicit provider before registering desktop bindings.
func ConfigureAppleCMS(a *App, raw string) error {
	if raw == "" {
		return nil
	}
	return configureAppleCMS(a, raw, newAppleCMSClient())
}

func configureAppleCMS(a *App, raw string, client *http.Client) error {
	u, err := parseAppleCMSURL(raw)
	if err != nil {
		return err
	}
	a.p = newAppleCMSProvider(*u, client)
	a.catalogCovers = newCatalogCoverService(client)
	return nil
}

func newAppleCMSProvider(u url.URL, client *http.Client) *applecms.Provider {
	return applecms.New(u, client, func(raw string) (provider.MediaSource, bool) {
		return catalogMediaSource(provider.MediaSource{Src: raw}, true, true)
	})
}

func parseAppleCMSURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, ok := catalogHTTPSURL(raw)
	if !ok || len(raw) > maxAppleCMSURLLength || strings.Contains(raw, "#") {
		return nil, errAppleCMSConfig
	}
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		return nil, errAppleCMSConfig
	}
	u.Host = strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api.php/provide/vod/"
		u.RawPath = ""
	}
	return u, nil
}

func parseAppleCMSUserURL(raw string) (*url.URL, error) {
	u, err := parseAppleCMSURL(raw)
	if err != nil || u.RawQuery != "" || u.ForceQuery || len(u.String()) > maxAppleCMSURLLength {
		return nil, errAppleCMSUserURL
	}
	return u, nil
}

func (a *App) savedAppleCMSURL() (string, error) {
	value, found, err := a.store.GetSetting(appleCMSURLKey)
	if err != nil {
		return "", errAppleCMSRead
	}
	if !found {
		return "", nil
	}
	var saved *string
	if err := json.Unmarshal([]byte(value), &saved); err != nil || saved == nil {
		return "", errAppleCMSStoredURL
	}
	if *saved == "" {
		return "", nil
	}
	u, err := parseAppleCMSUserURL(*saved)
	if err != nil {
		return "", errAppleCMSStoredURL
	}
	return u.String(), nil
}

func (a *App) AppleCMSConfiguration() AppleCMSConfiguration {
	a.awaitStartup()
	return a.appleCMSConfiguration()
}

func (a *App) appleCMSConfiguration() AppleCMSConfiguration {
	_, active := a.p.(*applecms.Provider)
	config := AppleCMSConfiguration{
		Active:       active,
		Managed:      a.appleCMSManaged,
		CanConfigure: a.store != nil && !a.appleCMSManaged,
	}
	if a.appleCMSManaged {
		if a.appleCMSStartupErr != nil {
			config.ReadError = a.appleCMSStartupErr.Error()
		}
		return config
	}
	if a.store == nil {
		config.ReadError = errAppleCMSRead.Error()
		return config
	}
	saved, err := a.savedAppleCMSURL()
	if err != nil {
		config.ReadError = err.Error()
		return config
	}
	config.SavedURL = saved
	config.RestartRequired = saved != a.appleCMSStartupURL
	if !active && saved != "" && a.appleCMSStartupErr != nil && !a.appleCMSSettingsChanged.Load() {
		config.ReadError = a.appleCMSStartupErr.Error()
	}
	return config
}

func (a *App) TestAppleCMSConnection(raw string) error {
	a.awaitStartup()
	if err := a.appleCMSConfigAvailable(); err != nil {
		return err
	}
	_, err := checkAppleCMSConnection(a.context(), raw, newAppleCMSClient())
	return err
}

func checkAppleCMSConnection(ctx context.Context, raw string, client *http.Client) (string, error) {
	defer client.CloseIdleConnections()
	u, err := parseAppleCMSUserURL(raw)
	if err != nil {
		return "", err
	}
	if _, err := newAppleCMSProvider(*u, client).CatalogPage(ctx, "", 1); err != nil {
		return "", errAppleCMSConnection
	}
	return u.String(), nil
}

func (a *App) SaveAppleCMSURL(raw string) (AppleCMSConfiguration, error) {
	a.awaitStartup()
	return a.saveAppleCMSURL(raw, newAppleCMSClient())
}

func (a *App) saveAppleCMSURL(raw string, client *http.Client) (AppleCMSConfiguration, error) {
	a.appleCMSConfigMu.Lock()
	defer a.appleCMSConfigMu.Unlock()
	if err := a.appleCMSConfigAvailable(); err != nil {
		return AppleCMSConfiguration{}, err
	}
	address, err := checkAppleCMSConnection(a.context(), raw, client)
	if err != nil {
		return AppleCMSConfiguration{}, err
	}
	return a.writeAppleCMSURL(address)
}

func (a *App) ClearAppleCMSURL() (AppleCMSConfiguration, error) {
	a.awaitStartup()
	a.appleCMSConfigMu.Lock()
	defer a.appleCMSConfigMu.Unlock()
	if err := a.appleCMSConfigAvailable(); err != nil {
		return AppleCMSConfiguration{}, err
	}
	return a.writeAppleCMSURL("")
}

func (a *App) appleCMSConfigAvailable() error {
	if a.appleCMSManaged {
		return errAppleCMSManaged
	}
	if a.store == nil {
		return ErrNoStore
	}
	return nil
}

func (a *App) writeAppleCMSURL(address string) (AppleCMSConfiguration, error) {
	value, _ := json.Marshal(address)
	if err := a.store.SetSetting(appleCMSURLKey, string(value)); err != nil {
		return AppleCMSConfiguration{}, errAppleCMSWrite
	}
	a.appleCMSSettingsChanged.Store(true)
	return a.appleCMSConfiguration(), nil
}

// CatalogPage addresses only the configured Apple CMS provider.
func (a *App) CatalogPage(query string, page int) (applecms.CatalogPage, error) {
	cms, ok := a.p.(*applecms.Provider)
	if !ok {
		return applecms.CatalogPage{}, provider.ErrNotAuthorized
	}
	result, err := cms.CatalogPage(a.context(), query, page)
	a.projectCatalogCovers(result.Entries)
	return result, err
}

func (a *App) CatalogGenres() ([]applecms.CatalogGenre, error) {
	ctx := a.context()
	cms, ok := a.p.(*applecms.Provider)
	if !ok || cms.Capabilities(ctx)["catalog"] != provider.Ready {
		return nil, provider.ErrNotAuthorized
	}
	return cms.CatalogGenres(ctx)
}

func (a *App) CatalogGenrePage(genre string, page int) (applecms.CatalogPage, error) {
	ctx := a.context()
	cms, ok := a.p.(*applecms.Provider)
	if !ok || cms.Capabilities(ctx)["catalog"] != provider.Ready {
		return applecms.CatalogPage{}, provider.ErrNotAuthorized
	}
	result, err := cms.CatalogGenrePage(ctx, genre, page)
	a.projectCatalogCovers(result.Entries)
	return result, err
}

// CatalogResumeIDs keeps the first twelve supplied bookmarks owned by the current CMS.
func (a *App) CatalogResumeIDs(ids []string) []string {
	result := make([]string, 0, 12)
	cms, ok := a.p.(*applecms.Provider)
	if !ok {
		return result
	}
	for _, id := range ids {
		if cms.OwnsDramaID(id) {
			result = append(result, id)
			if len(result) == cap(result) {
				break
			}
		}
	}
	return result
}
