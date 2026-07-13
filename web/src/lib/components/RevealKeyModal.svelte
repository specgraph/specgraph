<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { Button } from '$lib/components/ui/button/index.js';

  // Reveal props (plaintext/onClose) are preserved for backward compatibility.
  // Revoke props (revokeOpen/onRevoke) drive the destructive AlertDialog and are
  // wired by the /keys page (05-12); both default to a no-op closed state.
  let {
    plaintext = null,
    onClose,
    revokeOpen = $bindable(false),
    onRevoke,
  }: {
    plaintext?: string | null;
    onClose?: () => void;
    revokeOpen?: boolean;
    onRevoke?: () => void | Promise<void>;
  } = $props();

  let copied = $state(false);

  async function copy() {
    if (!plaintext) return;
    try {
      await navigator.clipboard.writeText(plaintext);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      copied = false;
    }
  }
</script>

<!-- Reveal path: the one-time plaintext key. It lives only in the parent's
     component-local reveal state; this component never persists or logs it
     (T-05-10). The {plaintext} binding is auto-escaped by Svelte (T-05-12). -->
<Dialog.Root open={!!plaintext} onOpenChange={(o) => { if (!o) onClose?.(); }}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>Your new API key</Dialog.Title>
      <Dialog.Description>
        This is the <strong class="font-semibold text-foreground">only</strong> time this
        secret is shown. Copy it now and store it in a secret manager — it cannot be
        recovered after you close this dialog.
      </Dialog.Description>
    </Dialog.Header>

    <div class="flex items-stretch gap-2">
      <code
        class="flex-1 break-all rounded-lg bg-muted px-3 py-2 font-mono text-xs leading-relaxed text-foreground"
        >{plaintext}</code
      >
      <Button type="button" variant="secondary" onclick={copy}>
        {copied ? 'Copied' : 'Copy'}
      </Button>
    </div>

    <p class="text-xs leading-relaxed text-muted-foreground">
      Store it as an environment variable your tooling reads, e.g.
      <code class="rounded bg-muted px-1 py-0.5 font-mono text-foreground">export SPECGRAPH_API_KEY=…</code>,
      or in your team's secret manager. Do not commit it to source control.
    </p>

    <Dialog.Footer>
      <Button type="button" onclick={() => onClose?.()}>I've stored it — close</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- Destructive revoke path (UI-SPEC Copywriting Contract). -->
<AlertDialog.Root bind:open={revokeOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Revoke this key?</AlertDialog.Title>
      <AlertDialog.Description>
        This permanently disables the key. Apps using it will stop working.
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action variant="destructive" onclick={() => onRevoke?.()}>
        Revoke
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
