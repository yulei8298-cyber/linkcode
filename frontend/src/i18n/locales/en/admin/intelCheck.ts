export default {
  intelCheck: {
    title: 'Model Intelligence Check',
    description:
      'Periodically sends one logic question and one drawing question to each checked group, using the same request shape as Codex CLI, and publishes the raw replies and judgment details on the public portal. Configure groups, questions and the review model before enabling.',
    tabs: { overview: 'Overview', targets: 'Groups', questions: 'Questions', settings: 'Settings' },

    common: {
      actions: 'Actions',
      enabled: 'Enabled',
      never: 'Never',
      none: 'None',
      logic: 'Logic',
      drawing: 'Drawing',
      bytes: '{n} bytes',
      loadError: 'Failed to load',
      saveSuccess: 'Saved',
      saveError: 'Failed to save',
      deleteSuccess: 'Deleted',
    },

    status: {
      pass: 'Pass',
      fail: 'Fail',
      request_error: 'Request error',
      running: 'Running',
      unknown: 'No data',
    },
    statusHint: {
      request_error:
        'A transport failure on our side. It does not count toward the degradation streak — it is not a claim about the checked model.',
    },

    effort: { low: 'Low', medium: 'Medium', high: 'High', xhigh: 'Extra high' },
    apiMode: { responses: 'Responses', chat_completions: 'Chat Completions' },
    matchMode: {
      exact: 'Exact',
      numeric: 'Numeric',
      contains: 'Contains',
      regex: 'Regex',
    },

    // ---------- Overview ----------
    overview: {
      runNow: 'Run a round now',
      runStarted: 'Started. Results will appear below once the round finishes.',
      runInFlight: 'A round is already running, please try again later',
      runFailed: 'Failed to start',
      rounds: 'Rounds',
      results: 'Results',
      roundColumns: {
        seq: 'Round',
        startedAt: 'Started',
        finishedAt: 'Finished',
        trigger: 'Trigger',
        questions: 'Questions',
      },
      resultColumns: {
        target: 'Group',
        kind: 'Kind',
        status: 'Status',
        latency: 'Latency',
        tokens: 'Tokens',
        error: 'Error',
        checkedAt: 'Checked at',
      },
      trigger: { cron: 'Scheduled', manual: 'Manual' },
      stillRunning: 'Running',
      noRounds: 'No checks yet',
      noRoundsHint:
        'Once groups and questions are configured and the feature is enabled, checks run automatically on schedule.',
      noResults: 'No matching results',
      filterAll: 'All',
      errorMessageHint: 'Admin-only column. The public page shows a neutral note instead.',
    },

    // ---------- Groups ----------
    targets: {
      create: 'New group',
      edit: 'Edit group',
      columns: {
        name: 'Name',
        model: 'Model',
        baseUrl: 'Base URL',
        apiKey: 'API Key',
        rateLabel: 'Rate label',
        sortOrder: 'Order',
      },
      empty: 'No groups yet',
      emptyHint: 'Add a group with its base URL, credential and model to include it in the checks.',
      deleteConfirm:
        'Delete group "{name}"? Its check history is deleted too, and the related cells disappear from the public page.',
      decryptFailed:
        'The stored credential cannot be decrypted (the encryption key may have changed). This group is skipped by the scheduler — please re-enter its API Key.',
      form: {
        name: 'Group name',
        namePlaceholder: 'e.g. Full-strength Codex',
        description: 'Description',
        descriptionPlaceholder: 'Shown on the group card of the public page',
        baseUrl: 'Base URL',
        baseUrlPlaceholder: 'https://example.com/v1',
        baseUrlHint:
          'Must include the version prefix (same convention as Codex CLI OPENAI_BASE_URL); /v1 is not appended for you.',
        apiKey: 'API Key',
        apiKeyPlaceholderCreate: 'Required',
        apiKeyPlaceholderEdit: 'Leave blank to keep the current key',
        apiMode: 'Request style',
        apiModeHint:
          'Codex CLI uses Responses, and the reasoning effort is only controllable in that mode.',
        model: 'Model',
        modelPlaceholder: 'e.g. gpt-5.4-codex',
        reasoningEffort: 'Reasoning effort',
        reasoningEffortHint:
          'Passed through verbatim, never silently downgraded — proving the full-strength configuration is the whole point.',
        rateLabel: 'Rate label',
        rateLabelPlaceholder: 'e.g. 1.0x',
        rateLabelHint: 'Display only on the public page; it never affects billing.',
        sortOrder: 'Sort order',
        sortOrderHint: 'Lower numbers come first.',
        enabled: 'Include in checks',
      },
    },

    // ---------- Questions ----------
    questions: {
      create: 'New question',
      edit: 'Edit question',
      columns: {
        title: 'Title',
        kind: 'Kind',
        answer: 'Expected answer',
        reference: 'Reference',
        rubric: 'Rubric',
      },
      empty: 'The question bank is empty',
      emptyHint:
        'Add at least one logic question. Drawing questions also need a reference artwork — it is the yardstick for the structural gate.',
      deleteConfirm:
        'Delete question "{title}"? Existing results are unaffected (the prompt is already snapshotted).',
      hasRubric: 'Configured',
      noRubric: 'Using built-in rubric',
      noReference: 'Not uploaded',
      form: {
        kind: 'Kind',
        kindHint:
          'The kind decides which judgment path is used. It can be changed later; fields of the other kind are cleared.',
        title: 'Title',
        titlePlaceholder: 'For admin identification only',
        prompt: 'Prompt',
        promptPlaceholder: 'Sent to the checked model verbatim',
        expectedAnswer: 'Expected answer',
        expectedAnswerHint: 'In regex mode this must be a valid regex; it is compiled on save.',
        matchMode: 'Match mode',
        referenceHtml: 'Reference HTML',
        referenceHtmlHint:
          'A full-strength sample. Its structural metrics become the denominator of the gate; leaving it empty skips every relative item, which effectively disables half the gate.',
        referenceMetrics: 'Reference metrics (computed server-side)',
        reviewRubric: 'Source review rubric',
        reviewRubricPlaceholder:
          'Leave empty to use the built-in rubric (subject completeness / structure / animation / detail / code quality)',
        drawingRules: 'Gate rules',
        minRatio: 'Minimum ratio',
        minRatioHint:
          'Each metric must be ≥ reference × this ratio. Default 0.7, derived from calibration; raising it is not recommended.',
        maxBytes: 'Max artifact size (bytes)',
        requiredKeywords: 'Required keywords',
        requiredKeywordsHint: 'Comma separated; leave empty to skip this check.',
        enabled: 'Include in rotation',
      },
      metrics: {
        shape_count: 'Shapes',
        animated_targets: 'Animated targets',
        defs_symbols: 'Reusable symbols',
        path_data_bytes: 'Path data',
        html_bytes: 'Artifact size',
        mechanisms: 'Animation mechanisms',
      },
    },

    // ---------- Calibration & dry run ----------
    trial: {
      evaluate: 'Calibrate',
      evaluateTitle: 'Judge a pasted artifact',
      evaluateHint:
        'Runs only the structural gate and source review — no drawing request is sent. Iterate with known-good and known-bad samples until the verdict matches your own judgment.',
      sourceLabel: 'Artifact HTML / SVG',
      sourcePlaceholder: 'Paste a complete HTML or SVG document',
      run: 'Run judgment',
      dryRun: 'Dry run',
      dryRunTitle: 'Dry run against one group',
      dryRunHint:
        'Sends a real request and judges it. Nothing is persisted and no round number is consumed. Uses the same judgment path as real checks.',
      selectTarget: 'Select a group',
      result: 'Verdict',
      gateItems: 'Structural gate items',
      reviewItems: 'Source review items',
      preview: 'Preview',
      source: 'Source',
      rawReply: 'Raw reply',
      noQuestion: 'Select a question first',
      noTarget: 'Select a group first',
      onlyDrawing: 'Only drawing questions support pasted-artifact judgment',
    },

    // ---------- Settings ----------
    settings: {
      enabled: 'Enable intelligence check',
      enabledHint:
        'When off, the public page returns 404 while admin configuration stays available. A review group and model must be set before enabling.',
      schedule: 'Schedule',
      intervalMinutes: 'Interval (minutes)',
      intervalHint:
        '5 to 1440. The interval doubles as the hard deadline for a single round (floored at request timeout + 1 minute).',
      requestTimeoutSeconds: 'Request timeout (seconds)',
      requestTimeoutHint:
        '30 to 900. At high reasoning effort, waiting minutes for the first byte is normal — do not set this too low.',
      concurrency: 'Concurrent groups',
      concurrencyHint:
        '1 to 32. The two questions of one group always run serially to avoid self-inflicted rate limiting.',
      retentionDays: 'Result retention (days)',

      judge: 'Source review',
      judgeTarget: 'Review group',
      judgeTargetHint: 'Reuses that group’s base URL and credential instead of maintaining another one.',
      judgeModel: 'Review model',
      judgeEffort: 'Review reasoning effort',
      passScore: 'Passing score',
      passScoreHint:
        '1 to 100, default 80. Both the structural gate and the source review must pass.',

      degraded: 'Degradation rule',
      failStreak: 'Consecutive failures to flag as degraded',
      recoverStreak: 'Consecutive passes to recover',
      degradedHint:
        'Logic questions only: drawing verdicts include a review score and fluctuate more, so including them causes frequent false alarms.',

      display: 'Public page',
      timelinePoints: 'Timeline cells',
      timelinePointsHint:
        '12 to 200. Note the statistics window is fixed at 24 hours regardless of cell count.',
      introTitle: 'Page title',
      introText: 'Page description',

      requireJudge: 'Set the review group and model before enabling',
    },
  },
}
