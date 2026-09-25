// Navigation and forms work without JavaScript. Only active command output polls.
const active = document.querySelector('[data-job]');
if (active) {
  const status = document.getElementById('connection-note');
  const update = async () => {
    if (document.hidden) { setTimeout(update, 2000); return; }
    try {
      const response = await fetch(`/jobs/${active.dataset.job}/status`, { cache: 'no-store', signal: AbortSignal.timeout(8000) });
      if (!response.ok) throw new Error('Status unavailable');
      const result = await response.json();
      if (!result.Active) { location.reload(); return; }
      document.getElementById('job-label').textContent = result.Label;
      document.getElementById('job-note').textContent = result.Note;
      document.getElementById('stdout').textContent = result.Stdout || 'No output yet.';
      document.getElementById('stderr').textContent = result.Stderr || 'No diagnostics.';
      status.textContent = '';
    } catch {
      status.textContent = 'Live updates are unavailable. Your command may still be running. Use Refresh status to check.';
    }
    setTimeout(update, 2000);
  };
  setTimeout(update, 1000);
}
