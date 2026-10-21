<script lang="ts">
  // Inviting one person: the copy to send them (a web page, or a file for the
  // app) and a message to go with it.
  import { judgeSteps } from '../lib/judge';
  import { invitation, type InviteKind } from '../lib/people';
  import type { Evaluator, Project } from '../lib/types';

  let {
    project,
    person,
    busy,
    error = '',
    onsend,
    onclose,
  }: {
    project: Project;
    person: Evaluator;
    busy: boolean;
    error?: string;
    onsend: (kind: InviteKind) => void;
    onclose: () => void;
  } = $props();

  let dialog = $state<HTMLDialogElement>();
  $effect(() => {
    if (dialog && !dialog.open) dialog.showModal();
  });

  let deadline = $state('');
  let kind = $state<InviteKind>('page');
  let copied = $state<boolean | null>(null);
  const nothing = $derived(judgeSteps(project, person).length === 0);
  // Signed by the first person, the one running the decision, unless they're still called "Me".
  const from = $derived(((name) => (name === 'Me' ? '' : name))(project.evaluators[0]?.name.trim() ?? ''));
  const message = $derived(invitation(project, person, { from, kind, deadline }));

  async function copy() {
    try {
      await navigator.clipboard.writeText(message);
      copied = true;
    } catch {
      copied = false; // not allowed here: the message can still be selected and copied by hand
    }
  }

  function send(k: InviteKind) {
    kind = k;
    onsend(k);
  }
</script>

<dialog bind:this={dialog} onclose={onclose} aria-labelledby="invite-title">
  <div class="body">
    <header>
      <h2 id="invite-title">Invite {person.name}</h2>
      <button class="btn quiet close" aria-label="Close" onclick={onclose}>✕</button>
    </header>

    {#if nothing}
      <p class="problem">
        {person.name} has nothing to answer yet: their role covers no comparisons or ratings. Change what they judge, or add criteria to
        score, then invite them.
      </p>
    {/if}
    {#if error}<p class="problem" role="alert">{error}</p>{/if}

    <div class="choices">
      <div class="choice" class:on={kind === 'page'}>
        <button class="btn primary" disabled={busy || nothing} onclick={() => send('page')}>Download web page</button>
        <p class="small muted">Recommended. {person.name} opens it in any browser and answers there; there's nothing to install.</p>
      </div>
      <div class="choice" class:on={kind === 'app'}>
        <button class="btn" disabled={busy || nothing} onclick={() => send('app')}>Download app file</button>
        <p class="small muted">For people who use AHP Matrix: they open it with Import file.</p>
      </div>
    </div>

    <label class="when small">
      Answer by
      <input class="field" type="date" bind:value={deadline} />
      <span class="muted">optional</span>
    </label>

    <label class="message">
      <span class="small">Message to send</span>
      <textarea class="field" rows="6" readonly value={message}></textarea>
    </label>
    {#if copied === false}<p class="small problem" role="alert">Couldn't copy here. Select the message above and copy it yourself.</p>{/if}
    <p class="small muted">Attach the downloaded file to an email or chat with this message. Their answers come back as a file to merge here.</p>
  </div>
  <footer>
    <button class="btn" onclick={copy}>{copied ? 'Copied' : 'Copy message'}</button>
    <button class="btn quiet" onclick={onclose}>Done</button>
  </footer>
</dialog>

<style>
  dialog {
    width: min(36rem, calc(100vw - 2rem));
    max-height: calc(100vh - 2rem);
    padding: 0;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-m);
    background: var(--paper);
    color: var(--ink);
    box-shadow: var(--shadow-lift);
  }
  dialog[open] {
    display: flex;
    flex-direction: column;
  }
  dialog::backdrop {
    background: rgb(16 22 29 / 0.55);
  }
  .body {
    display: grid;
    gap: 1rem;
    padding: 1.25rem 1.4rem 1rem;
    overflow-y: auto;
    min-height: 0;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: start;
    gap: 1rem;
  }
  .close {
    padding: 0.2rem 0.5rem;
  }
  .problem {
    margin: 0;
    padding: 0.6rem 0.8rem;
    border-radius: var(--radius-s);
    background: var(--rust-wash);
  }
  .choices {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: 0.75rem;
  }
  .choice {
    display: grid;
    gap: 0.4rem;
    align-content: start;
    justify-items: start;
    padding: 0.8rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-m);
  }
  .choice.on {
    border-color: var(--brass);
    background: var(--brass-wash);
  }
  .choice p {
    margin: 0;
  }
  .when {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .when input {
    width: auto;
  }
  .message {
    display: grid;
    gap: 0.3rem;
  }
  .message textarea {
    resize: vertical;
    font-size: 0.92rem;
    line-height: 1.45;
  }
  footer {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0.85rem 1.4rem 1.1rem;
    border-top: 1px solid var(--line);
  }
</style>
