package app

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"jindi-drama/internal/provider/licensed"
)

const licensedDirectoryKey = "licensed_media_directory"

type LicensedConfiguration struct {
	StartupDirectory string `json:"startupDirectory"`
	ActiveDirectory  string `json:"activeDirectory"`
	SavedDirectory   string `json:"savedDirectory"`
	Managed          bool   `json:"managed"`
	CanConfigure     bool   `json:"canConfigure"`
	RestartRequired  bool   `json:"restartRequired"`
	ReadError        string `json:"readError,omitempty"`
}

// PrepareLicensedDirectory loads the effective directory before the asset server starts.
func PrepareLicensedDirectory(a *App, managedDirectory string) (string, error) {
	if a.store == nil {
		err := a.openStore("")
		if err != nil && managedDirectory == "" {
			return "", errors.New("无法读取本地片库设置")
		}
	}
	a.licensedManaged = managedDirectory != ""
	if a.licensedManaged {
		a.licensedDirectory = managedDirectory
		return managedDirectory, nil
	}
	directory, err := a.savedLicensedDirectory()
	if err != nil {
		return "", err
	}
	a.licensedDirectory = directory
	return directory, nil
}

func (a *App) savedLicensedDirectory() (string, error) {
	if a.store == nil {
		return "", ErrNoStore
	}
	raw, ok, err := a.store.GetSetting(licensedDirectoryKey)
	if err != nil {
		return "", errors.New("无法读取本地片库设置")
	}
	if !ok {
		return "", nil
	}
	var path string
	if err := json.Unmarshal([]byte(raw), &path); err != nil {
		return "", errors.New("本地片库设置格式无效")
	}
	return path, nil
}

func (a *App) LicensedConfiguration() LicensedConfiguration {
	config := LicensedConfiguration{
		StartupDirectory: a.licensedDirectory,
		Managed:          a.licensedManaged,
		CanConfigure:     a.store != nil && !a.licensedManaged,
	}
	if a.licensed != nil {
		config.ActiveDirectory = a.licensedDirectory
	}
	if !config.CanConfigure {
		return config
	}
	saved, err := a.savedLicensedDirectory()
	if err != nil {
		config.ReadError = err.Error()
		return config
	}
	config.SavedDirectory = saved
	config.RestartRequired = saved != a.licensedDirectory || (a.licensedErr != nil && a.licensedDirectory == "")
	return config
}

func (a *App) ChooseLicensedDirectory() (LicensedConfiguration, error) {
	if a.ctx == nil {
		return LicensedConfiguration{}, errors.New("无法打开系统目录选择器")
	}
	return a.chooseLicensedDirectory(func() (string, error) {
		return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
			Title: "选择本地片库目录",
		})
	})
}

func (a *App) chooseLicensedDirectory(pick func() (string, error)) (LicensedConfiguration, error) {
	a.licensedConfigMu.Lock()
	defer a.licensedConfigMu.Unlock()
	if err := a.licenseConfigAvailable(); err != nil {
		return LicensedConfiguration{}, err
	}
	selected, err := pick()
	if err != nil {
		return LicensedConfiguration{}, errors.New("无法打开系统目录选择器")
	}
	if selected == "" {
		return a.LicensedConfiguration(), nil
	}
	directory, err := filepath.Abs(selected)
	if err != nil {
		return LicensedConfiguration{}, errors.New("无法读取所选目录")
	}
	media, err := licensed.New(directory)
	if err != nil {
		return LicensedConfiguration{}, err
	}
	_ = media.Close()
	return a.saveLicensedDirectory(directory)
}

func (a *App) ClearLicensedDirectory() (LicensedConfiguration, error) {
	a.licensedConfigMu.Lock()
	defer a.licensedConfigMu.Unlock()
	if err := a.licenseConfigAvailable(); err != nil {
		return LicensedConfiguration{}, err
	}
	return a.saveLicensedDirectory("")
}

func (a *App) licenseConfigAvailable() error {
	if a.licensedManaged {
		return errors.New("本地片库目录由运营方配置")
	}
	if a.store == nil {
		return ErrNoStore
	}
	return nil
}

func (a *App) saveLicensedDirectory(directory string) (LicensedConfiguration, error) {
	value, _ := json.Marshal(directory)
	if err := a.store.SetSetting(licensedDirectoryKey, string(value)); err != nil {
		return LicensedConfiguration{}, errors.New("无法保存本地片库设置")
	}
	return a.LicensedConfiguration(), nil
}
