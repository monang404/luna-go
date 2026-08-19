package codeproject

import (
	"context"
	"os"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// genSysPrompt mirrors aiproject's gen_sysprompt (10-project_generate.zsh).
const genSysPrompt = `Kamu programmer expert. Buat project multi-file sesuai request. WAJIB format tiap file dengan penanda persis: ### FILE: nama_file.ext lalu isi kode di baris berikutnya, tanpa markdown/backtick. Pisahkan tiap file dengan penanda itu. WAJIB tulis SEMUA file yang direferensikan lewat import di project ini, jangan skip satupun. WAJIB pakai baris baru SUNGGUHAN buat pisah tiap statement/baris kode di semua file di luar string. KALAU request yang dikasih SUDAH berupa spec terstruktur (ada header [APLIKASI]/[FILES]/[ALUR]/dst, biasanya hasil dari 'luna spec'), WAJIB ikuti persis: nama file di [FILES] harus jadi nama file beneran, tanggung jawab tiap file harus sesuai deskripsinya, dan import antar file harus konsisten sama yang disebut di situ. Kalau ada [CONTOH_INPUT], pastikan urutan & jumlah input() di main.py sesuai urutan itu. Kalau project-nya butuh banyak file/kode panjang, WAJIB prioritaskan nulis SEMUA file secara lengkap dulu.`

const formatReminder = "\n\nINGAT: jawaban HARUS dimulai dari baris '### FILE: <nama_file>' -- jangan tulis penjelasan/narasi apapun sebelum penanda itu, dan jangan ada satupun file yang ditulis tanpa penanda itu di depannya."

// GenerateResult is GenerateProject's return shape (the Go equivalent
// of _ai_project_generate's caller-visible $logfile content plus its
// $has_markers/$generation_ok out-params).
type GenerateResult struct {
	RawLog       string
	HasMarkers   bool
	GenerationOK bool
	AttemptsMade int
}

// GenerateProject mirrors _ai_project_generate(prompt, maxTries,
// logfile): retry generation up to maxTries times, adding an explicit
// format reminder from the 2nd attempt onward, writing each raw reply
// to logfile (overwriting previous attempts, matching the zsh source's
// `> "$logfile"` redirection), running SanitizePyCode's
// "--normalize-markers" mode on it, then stopping as soon as a
// "### FILE: " marker is found (or maxTries is exhausted).
func (s *Service) GenerateProject(ctx context.Context, prompt, logfile string, maxTries int) GenerateResult {
	if maxTries < 1 {
		maxTries = 2
	}
	result := GenerateResult{}

	for tries := 0; tries < maxTries; tries++ {
		result.AttemptsMade = tries + 1
		thisPrompt := prompt
		if tries > 0 {
			thisPrompt = prompt + formatReminder
		}
		res, err := s.Requester.Complete(ctx, genSysPrompt, thisPrompt, config.TaskSmart, config.TaskProviderOrderBig, projectMaxToks)
		if err != nil || res.Content == "" {
			continue
		}
		result.GenerationOK = true
		result.RawLog = res.Content
		writeLogfile(logfile, res.Content)
		aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, logfile, "--normalize-markers")

		if content, ok := readLogfile(logfile); ok {
			result.RawLog = content
		}
		if containsFileMarker(result.RawLog) {
			result.HasMarkers = true
			break
		}
	}
	return result
}

func writeLogfile(path, content string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(content), 0o644)
}

func readLogfile(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func containsFileMarker(s string) bool {
	for _, line := range splitLines(s) {
		if len(line) >= len(fileMarkerPrefix) && line[:len(fileMarkerPrefix)] == fileMarkerPrefix {
			return true
		}
	}
	return false
}

const fileMarkerPrefix = "### FILE: "
