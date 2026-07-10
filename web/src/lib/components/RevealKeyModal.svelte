<script lang="ts">
  let { plaintext, onClose }: { plaintext: string; onClose: () => void } = $props();

  let copied = $state(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(plaintext);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      copied = false;
    }
  }
</script>

<div class="overlay">
  <div class="reveal-card" role="dialog" aria-modal="true" aria-labelledby="reveal-title">
    <h2 id="reveal-title">Your new API key</h2>
    <p class="warn">
      This is the <strong>only</strong> time this secret is shown. Copy it now and store it in a
      secret manager — it cannot be recovered after you close this dialog.
    </p>

    <div class="secret-row">
      <code class="secret">{plaintext}</code>
      <button type="button" class="copy" onclick={copy}>{copied ? 'Copied' : 'Copy'}</button>
    </div>

    <p class="hint">
      Store it as an environment variable your tooling reads, e.g.
      <code>export SPECGRAPH_API_KEY=…</code>, or in your team's secret manager. Do not commit it to
      source control.
    </p>

    <button type="button" class="done" onclick={onClose}>I've stored it — close</button>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .reveal-card {
    background: white;
    border-radius: 8px;
    padding: 2rem;
    width: 440px;
    max-width: 90vw;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
  }
  h2 {
    margin: 0 0 0.5rem;
    color: #1a1a2e;
    font-size: 1.1rem;
  }
  .warn {
    color: #92400e;
    background: #fef3c7;
    border-radius: 4px;
    padding: 0.5rem 0.75rem;
    font-size: 0.85rem;
    margin: 0 0 1rem;
  }
  .secret-row {
    display: flex;
    gap: 0.5rem;
    align-items: stretch;
    margin-bottom: 1rem;
  }
  .secret {
    flex: 1;
    display: block;
    padding: 0.5rem 0.75rem;
    background: #0f172a;
    color: #e2e8f0;
    border-radius: 4px;
    font-family: 'SF Mono', ui-monospace, Menlo, monospace;
    font-size: 0.8rem;
    word-break: break-all;
    line-height: 1.4;
  }
  .copy {
    flex: 0 0 auto;
    padding: 0.5rem 0.9rem;
    background: #1a1a2e;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .copy:hover {
    background: #2d2d4e;
  }
  .hint {
    color: #64748b;
    font-size: 0.8rem;
    margin: 0 0 1.25rem;
    line-height: 1.5;
  }
  .hint code {
    background: #f1f5f9;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    font-size: 0.78rem;
    color: #334155;
  }
  .done {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .done:hover {
    background: #2d2d4e;
  }
</style>
