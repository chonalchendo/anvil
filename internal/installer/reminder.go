package installer

// MergeReminderSuppression writes env vars to settingsPath that reduce
// system-reminder noise in Claude Code sessions. Specifically it disables
// prompt suggestions, which are the harness-side reminder source anvil can
// suppress via settings.json. Other reminder sources (skills-dump cadence,
// TaskCreate nudge) are outside this scope — the harness does not expose a
// settings knob for them.
//
// Returns changed=false when the suppression settings are already present.
func MergeReminderSuppression(settingsPath string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	env := getOrCreateMap(settings, "env")
	// CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false disables the harness-side
	// prompt-suggestion reminders that fire on cadence regardless of session
	// context, contributing to per-turn reminder noise.
	const key = "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"
	const want = "false"
	if env[key] == want {
		return false, nil
	}
	env[key] = want
	settings["env"] = env

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveReminderSuppression removes the env vars written by
// MergeReminderSuppression. Missing file or key is not an error.
func RemoveReminderSuppression(settingsPath string) (bool, error) {
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return false, err
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		return false, nil
	}
	const key = "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"
	if _, present := env[key]; !present {
		return false, nil
	}
	delete(env, key)
	settings["env"] = env

	if err := writeSettings(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}
