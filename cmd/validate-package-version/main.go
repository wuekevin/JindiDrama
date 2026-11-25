package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"jindi-drama/internal/update"
)

func main() {
	config := flag.String("wails-config", "", "写入系统可用的数字版本，并输出该版本")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	core, err := platformVersion(flag.Arg(0))
	if err == nil && *config != "" {
		err = stampWailsVersion(*config, core)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *config != "" {
		fmt.Println(core)
	}
}

func platformVersion(version string) (string, error) {
	// The desktop startup marker retains at most 128 version bytes.
	if len(version) > 128 {
		return "", errors.New("发行版本不得超过 128 字符，以便精确验证启动版本")
	}
	if !update.ValidPackageVersion(version) {
		return "", errors.New("版本必须为 v 前缀加完整的 SemVer，例如 v1.2.3 或 v1.2.3-rc.2")
	}
	numeric := strings.TrimPrefix(version, "v")
	if i := strings.IndexAny(numeric, "-+"); i >= 0 {
		numeric = numeric[:i]
	}
	for _, component := range strings.Split(numeric, ".") {
		if _, err := strconv.ParseUint(component, 10, 16); err != nil {
			return "", errors.New("发行版本每个数字必须在 Windows 版本元数据支持的 0–65535 范围内")
		}
	}
	return numeric, nil
}

func stampWailsVersion(path, core string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var options map[string]json.RawMessage
	if err := json.Unmarshal(existing, &options); err != nil {
		return err
	}
	if options == nil {
		return errors.New("Wails 配置必须是 JSON 对象")
	}
	infoKey := "info"
	if options[infoKey] == nil && options["Info"] != nil {
		infoKey = "Info"
	}
	info := map[string]json.RawMessage{}
	if raw := options[infoKey]; len(raw) != 0 {
		if err := json.Unmarshal(raw, &info); err != nil {
			return err
		}
	}
	if info == nil {
		info = map[string]json.RawMessage{}
	}
	info["productVersion"] = json.RawMessage(strconv.Quote(core))
	options[infoKey], err = json.Marshal(info)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o600)
}
