// Progress store: owns the SSE lifecycle and the reactive progress state that
// ProgressCard + the download UI render. App.svelte drives upload; this module
// drives everything from the jobId onward.
//
// The codebase uses Svelte 5 runes throughout (no svelte/store), so this is a
// runes-based store in a .svelte.js module — the extension is what lets $state
// compile outside a component.
//
// Backend contract note: the SSE `pct` field is WHOLE-JOB progress
// (done / (files × presets)), and `preset` only names the unit that just
// finished. There is no per-preset percentage on the wire, so a preset's own
// progress is derived from how many of its expected events have arrived:
// expected = fileCount, completed bumps once per `processing` event for it.

export function createProgress() {
  // 'idle' | 'uploading' | 'processing' | 'done' | 'error'
  let status = $state('idle');
  // { [presetName]: { expected, completed } }
  let presetProgress = $state({});
  let downloadUrl = $state(null);
  let errorMessage = $state(null);

  let eventSource = null;

  function close() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function fail(message) {
    close();
    errorMessage = message;
    status = 'error';
  }

  // start seeds per-preset counters before the stream opens. fileCount is how
  // many events each preset will emit (one per uploaded file).
  function start(presetNames, fileCount) {
    errorMessage = null;
    downloadUrl = null;
    const next = {};
    for (const name of presetNames) {
      next[name] = { expected: fileCount, completed: 0 };
    }
    presetProgress = next;
    status = 'uploading';
  }

  // uploaded flips state from 'uploading' to 'processing' once the POST /upload
  // response is in hand and we're about to open the stream.
  function connect(jobId) {
    close();
    status = 'processing';
    eventSource = new EventSource(`/progress/${jobId}`);

    eventSource.onmessage = (e) => {
      let event;
      try {
        event = JSON.parse(e.data);
      } catch {
        return;
      }

      if (event.status === 'complete') {
        downloadUrl = event.downloadUrl ?? `/download/${jobId}`;
        status = 'done';
        close();
      } else if (event.status === 'error') {
        fail('Processing failed');
      } else if (event.status === 'processing') {
        const entry = presetProgress[event.preset];
        if (entry) {
          // Reassign for reactivity; cap at expected as a safety net.
          presetProgress = {
            ...presetProgress,
            [event.preset]: {
              ...entry,
              completed: Math.min(entry.completed + 1, entry.expected),
            },
          };
        }
      }
    };

    eventSource.onerror = () => {
      // A clean close after completion also fires onerror; ignore once done.
      if (status === 'processing') fail('Lost connection to the server');
    };
  }

  function reset() {
    close();
    status = 'idle';
    presetProgress = {};
    downloadUrl = null;
    errorMessage = null;
  }

  return {
    get status() {
      return status;
    },
    get presetProgress() {
      return presetProgress;
    },
    get downloadUrl() {
      return downloadUrl;
    },
    get errorMessage() {
      return errorMessage;
    },
    start,
    connect,
    reset,
    close,
    fail,
  };
}
