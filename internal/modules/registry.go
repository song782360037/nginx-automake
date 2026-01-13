package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Module struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
	Flag        string `json:"flag"`
	Path        string `json:"path,omitempty"`
}

type Registry struct {
	modules map[string]Module
}

var validModuleName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func LoadRegistry(data []byte) (*Registry, error) {
	var list []Module
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析模块配置失败: %w", err)
	}
	modules := make(map[string]Module, len(list))
	for _, mod := range list {
		modules[mod.Name] = mod
	}
	return &Registry{modules: modules}, nil
}

func (r *Registry) List() []Module {
	list := make([]Module, 0, len(r.modules))
	for _, mod := range r.modules {
		list = append(list, mod)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.Compare(list[i].Name, list[j].Name) < 0
	})
	return list
}

func (r *Registry) Get(name string) (Module, bool) {
	mod, ok := r.modules[name]
	return mod, ok
}

func ValidateCustomModule(name, repo, flag string) (Module, error) {
	if strings.TrimSpace(name) == "" {
		return Module{}, errors.New("模块名称不能为空")
	}
	if !validModuleName.MatchString(name) {
		return Module{}, errors.New("模块名称仅支持字母、数字、点、下划线和短横线")
	}
	if strings.TrimSpace(repo) == "" {
		return Module{}, errors.New("模块仓库地址不能为空")
	}
	if !strings.HasPrefix(repo, "https://") {
		return Module{}, errors.New("仅支持 https Git 仓库地址")
	}
	if flag == "" {
		flag = "add-module"
	}
	if flag != "add-module" && flag != "add-dynamic-module" {
		return Module{}, errors.New("模块类型仅支持 add-module 或 add-dynamic-module")
	}
	return Module{Name: name, Repo: repo, Flag: flag}, nil
}

func ResolveModulePath(mod Module, modulesDir string, workDir string) (string, error) {
	if mod.Path != "" {
		if filepath.IsAbs(mod.Path) {
			return mod.Path, nil
		}
		return filepath.Join(modulesDir, mod.Path), nil
	}
	if mod.Repo == "" {
		return "", fmt.Errorf("模块 %s 没有仓库地址", mod.Name)
	}
	cloneDir := filepath.Join(workDir, "modules", mod.Name)
	if err := os.MkdirAll(filepath.Dir(cloneDir), 0o755); err != nil {
		return "", err
	}
	return cloneDir, nil
}

func ModuleFlag(mod Module) string {
	if mod.Flag == "add-dynamic-module" {
		return "--add-dynamic-module"
	}
	return "--add-module"
}
