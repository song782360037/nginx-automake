package job

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"nginx-automake/internal/modules"
	"nginx-automake/internal/parser"
)

type Status string

type StepStatus string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"

	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

type Step struct {
	Name    string     `json:"name"`
	Status  StepStatus `json:"status"`
	Message string     `json:"message"`
}

type Job struct {
	ID           string              `json:"id"`
	CreatedAt    time.Time           `json:"createdAt"`
	Status       Status              `json:"status"`
	Steps        []Step              `json:"steps"`
	Logs         []string            `json:"logs"`
	Error        string              `json:"error"`
	ArtifactPath string              `json:"artifactPath"`
	Script       string              `json:"script"`
	Result       *parser.ParseResult `json:"result"`
	Request      BuildRequest        `json:"request"`
}

type BuildRequest struct {
	Output        string            `json:"output"`
	ModuleNames   []string          `json:"moduleNames"`
	CustomModules []CustomModuleReq `json:"customModules"`
	TargetVersion string            `json:"targetVersion"`
}

type CustomModuleReq struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	Flag string `json:"flag"`
}

type Queue struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	queue      chan *Job
	workers    int
	modulesDir string
	workRoot   string
	registry   *modules.Registry
	timeout    time.Duration
	history    *HistoryStore
}

func NewQueue(workers int, modulesDir, workRoot string, registry *modules.Registry, timeout time.Duration, history *HistoryStore) *Queue {
	return &Queue{
		jobs:       make(map[string]*Job),
		queue:      make(chan *Job, 100),
		workers:    workers,
		modulesDir: modulesDir,
		workRoot:   workRoot,
		registry:   registry,
		timeout:    timeout,
		history:    history,
	}
}

func (q *Queue) Start() {
	for i := 0; i < q.workers; i++ {
		go q.worker()
	}
}

func (q *Queue) Enqueue(req BuildRequest) (*Job, error) {
	jobID, err := randomID()
	if err != nil {
		return nil, err
	}
	job := &Job{
		ID:        jobID,
		CreatedAt: time.Now(),
		Status:    StatusQueued,
		Steps: []Step{
			{Name: "解析配置", Status: StepPending},
			{Name: "准备源代码", Status: StepPending},
			{Name: "准备模块", Status: StepPending},
			{Name: "执行编译", Status: StepPending},
			{Name: "整理产物", Status: StepPending},
		},
		Request: req,
	}
	q.mu.Lock()
	q.jobs[jobID] = job
	q.mu.Unlock()
	q.queue <- job
	return job, nil
}

func (q *Queue) Get(id string) (*Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	return job, ok
}

func (q *Queue) worker() {
	for job := range q.queue {
		q.updateStatus(job.ID, StatusRunning)
		ctx := context.Background()
		var cancel context.CancelFunc
		if q.timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, q.timeout)
		}
		if err := q.runJob(ctx, job); err != nil {
			q.failJob(job.ID, err)
		} else {
			q.updateStatus(job.ID, StatusSuccess)
		}
		if cancel != nil {
			cancel()
		}
	}
}

func (q *Queue) runJob(ctx context.Context, job *Job) error {
	parsed, err := parser.ParseNginxV(job.Request.Output)
	if err != nil {
		q.setStep(job.ID, "解析配置", StepFailed, err.Error())
		return err
	}
	if strings.TrimSpace(job.Request.TargetVersion) != "" {
		parsed.Version = strings.TrimSpace(job.Request.TargetVersion)
	}
	q.setStep(job.ID, "解析配置", StepSuccess, "解析完成")
	job.Result = parsed

	workDir := filepath.Join(q.workRoot, job.ID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		q.setStep(job.ID, "准备源代码", StepFailed, err.Error())
		return err
	}

	nginxTar := filepath.Join(workDir, fmt.Sprintf("nginx-%s.tar.gz", parsed.Version))
	srcDir := filepath.Join(workDir, fmt.Sprintf("nginx-%s", parsed.Version))

	q.setStep(job.ID, "准备源代码", StepRunning, "下载 Nginx 源码")
	if err := q.runCommand(ctx, job.ID, workDir, "curl", "-fSL", fmt.Sprintf("https://nginx.org/download/nginx-%s.tar.gz", parsed.Version), "-o", nginxTar); err != nil {
		q.setStep(job.ID, "准备源代码", StepFailed, err.Error())
		return err
	}
	if err := q.runCommand(ctx, job.ID, workDir, "tar", "-xzf", nginxTar); err != nil {
		q.setStep(job.ID, "准备源代码", StepFailed, err.Error())
		return err
	}
	q.setStep(job.ID, "准备源代码", StepSuccess, "源码就绪")

	q.setStep(job.ID, "准备模块", StepRunning, "同步模块")
	moduleArgs, err := q.prepareModules(ctx, job, workDir)
	if err != nil {
		q.setStep(job.ID, "准备模块", StepFailed, err.Error())
		return err
	}
	q.setStep(job.ID, "准备模块", StepSuccess, "模块就绪")

	q.setStep(job.ID, "执行编译", StepRunning, "执行 configure")
	configureArgs := q.composeConfigureArgs(parsed.Arguments, moduleArgs)
	job.Script = buildScript(parsed.Version, configureArgs)
	if err := q.runCommand(ctx, job.ID, srcDir, "./configure", configureArgs...); err != nil {
		q.setStep(job.ID, "执行编译", StepFailed, err.Error())
		return err
	}
	q.appendLog(job.ID, "configure 完成，开始编译")
	if err := q.runCommand(ctx, job.ID, srcDir, "make", "-j", fmt.Sprintf("%d", max(1, int64(runtime.NumCPU())))); err != nil {
		q.setStep(job.ID, "执行编译", StepFailed, err.Error())
		return err
	}
	q.setStep(job.ID, "执行编译", StepSuccess, "编译完成")

	q.setStep(job.ID, "整理产物", StepRunning, "整理 nginx 二进制")
	artifactDir := filepath.Join(workDir, "artifact")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		q.setStep(job.ID, "整理产物", StepFailed, err.Error())
		return err
	}
	srcBinary := filepath.Join(srcDir, "objs", "nginx")
	artifact := filepath.Join(artifactDir, fmt.Sprintf("nginx-%s", parsed.Version))
	if err := copyFile(srcBinary, artifact); err != nil {
		q.setStep(job.ID, "整理产物", StepFailed, err.Error())
		return err
	}
	job.ArtifactPath = artifact
	q.setStep(job.ID, "整理产物", StepSuccess, "产物已生成")
	if q.history != nil {
		entry := HistoryEntry{
			ID:        job.ID,
			CreatedAt: job.CreatedAt,
			Version:   parsed.Version,
			Modules:   append([]string{}, job.Request.ModuleNames...),
			Status:    string(StatusSuccess),
			Artifact:  artifact,
		}
		_ = q.history.Append(entry)
	}
	return nil
}

func (q *Queue) composeConfigureArgs(original []string, moduleArgs []string) []string {
	filtered := make([]string, 0, len(original))
	for _, arg := range original {
		if strings.HasPrefix(arg, "--add-module=") || strings.HasPrefix(arg, "--add-dynamic-module=") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return append(filtered, moduleArgs...)
}

func (q *Queue) prepareModules(ctx context.Context, job *Job, workDir string) ([]string, error) {
	var moduleArgs []string
	for _, name := range job.Request.ModuleNames {
		mod, ok := q.registry.Get(name)
		if !ok {
			return nil, fmt.Errorf("模块 %s 未在预设列表中", name)
		}
		modulePath, err := modules.ResolveModulePath(mod, q.modulesDir, workDir)
		if err != nil {
			return nil, err
		}
		if mod.Path == "" {
			if err := q.cloneModule(ctx, job.ID, mod.Repo, modulePath); err != nil {
				return nil, err
			}
		} else if _, err := os.Stat(modulePath); err != nil {
			if mod.Repo == "" {
				return nil, fmt.Errorf("预置模块 %s 未找到，请提前下载到 %s", name, modulePath)
			}
			if err := q.cloneModule(ctx, job.ID, mod.Repo, modulePath); err != nil {
				return nil, err
			}
		}
		moduleArgs = append(moduleArgs, fmt.Sprintf("%s=%s", modules.ModuleFlag(mod), modulePath))
	}

	for _, custom := range job.Request.CustomModules {
		mod, err := modules.ValidateCustomModule(custom.Name, custom.Repo, custom.Flag)
		if err != nil {
			return nil, err
		}
		modulePath, err := modules.ResolveModulePath(mod, q.modulesDir, workDir)
		if err != nil {
			return nil, err
		}
		if err := q.cloneModule(ctx, job.ID, mod.Repo, modulePath); err != nil {
			return nil, err
		}
		moduleArgs = append(moduleArgs, fmt.Sprintf("%s=%s", modules.ModuleFlag(mod), modulePath))
	}

	return moduleArgs, nil
}

func (q *Queue) cloneModule(ctx context.Context, jobID, repo, dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	return q.runCommand(ctx, jobID, filepath.Dir(dir), "git", "clone", "--depth", "1", repo, dir)
}

func (q *Queue) runCommand(ctx context.Context, jobID, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go q.streamOutput(jobID, stdout, &wg)
	go q.streamOutput(jobID, stderr, &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (q *Queue) streamOutput(jobID string, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		q.appendLog(jobID, scanner.Text())
	}
}

func (q *Queue) appendLog(jobID, line string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[jobID]
	if !ok {
		return
	}
	if len(job.Logs) > 2000 {
		job.Logs = job.Logs[len(job.Logs)-1500:]
	}
	job.Logs = append(job.Logs, line)
}

func (q *Queue) setStep(jobID, name string, status StepStatus, message string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[jobID]
	if !ok {
		return
	}
	for i := range job.Steps {
		if job.Steps[i].Name == name {
			job.Steps[i].Status = status
			job.Steps[i].Message = message
			break
		}
	}
}

func (q *Queue) updateStatus(jobID string, status Status) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if job, ok := q.jobs[jobID]; ok {
		job.Status = status
	}
}

func (q *Queue) failJob(jobID string, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[jobID]
	if !ok {
		return
	}
	job.Status = StatusFailed
	job.Error = err.Error()
	if q.history != nil && job.Result != nil {
		entry := HistoryEntry{
			ID:        job.ID,
			CreatedAt: job.CreatedAt,
			Version:   job.Result.Version,
			Modules:   append([]string{}, job.Request.ModuleNames...),
			Status:    string(job.Status),
			Error:     job.Error,
		}
		_ = q.history.Append(entry)
	}
}

func randomID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}

func max(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func buildScript(version string, configureArgs []string) string {
	return fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\n\nVERSION=%s\nWORKDIR=./build-$VERSION\n\nmkdir -p $WORKDIR\ncd $WORKDIR\n\ncurl -fSL https://nginx.org/download/nginx-$VERSION.tar.gz -o nginx.tar.gz\ntar -xzf nginx.tar.gz\ncd nginx-$VERSION\n\n./configure %s\nmake -j$(nproc)\n\ncp objs/nginx ./nginx-$VERSION\n", version, strings.Join(configureArgs, " "))
}

func (q *Queue) ValidateRequest(req BuildRequest) error {
	if strings.TrimSpace(req.Output) == "" {
		return errors.New("nginx -V 输出不能为空")
	}
	if strings.TrimSpace(req.TargetVersion) != "" && !parser.ValidVersion(req.TargetVersion) {
		return errors.New("目标版本号格式不正确，例如 1.24.0")
	}
	return nil
}
