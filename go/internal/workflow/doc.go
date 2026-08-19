// Package workflow ports 30-luna/40-workflow/ (aicommit, aiplan,
// aiprompt, aispec, aibuild, aireview + _ai_review_diff_core,
// aisummarize) into Go, per SESSION-54.
//
// aibuild composes internal/workflow's own Spec with
// internal/codeproject's Project -- exactly like the zsh source, which
// calls aispec then aiproject in sequence. This is the one place in
// SESSION-54's four packages that depends on another of the four; it's
// a one-way dependency (workflow -> codeproject), matching the zsh
// source's own load order (30-code/ loads before 40-workflow/).
package workflow
