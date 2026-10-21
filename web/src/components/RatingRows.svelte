<script lang="ts">
  // One row per option with a 1–10 rating; choosing the current rating again clears it.
  let {
    items,
    get,
    set,
  }: {
    items: { id: string; name: string; color: string }[];
    get: (id: string) => number | undefined;
    set: (id: string, v: number | undefined) => void;
  } = $props();
</script>

<ul class="rows">
  {#each items as a (a.id)}
    {@const r = get(a.id)}
    <li>
      <span class="who"><span class="dot" style:background={a.color}></span>{a.name}</span>
      <div class="seg" role="radiogroup" aria-label="Rating for {a.name}">
        {#each [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as n}
          <button role="radio" aria-checked={r === n} class:on={r === n} onclick={() => set(a.id, r === n ? undefined : n)}>{n}</button>
        {/each}
      </div>
    </li>
  {/each}
</ul>

<style>
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
  .rows li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem 1rem;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--line);
  }
  .who {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 600;
  }
  .dot {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 50%;
  }
  .seg {
    display: grid;
    grid-template-columns: repeat(10, minmax(1.9rem, 2.1rem));
    max-width: 100%;
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-s);
    overflow: hidden;
  }
  .seg button {
    border: 0;
    border-right: 1px solid var(--line);
    padding: 0.4rem 0;
    background: var(--paper);
    cursor: pointer;
    font-weight: 600;
    color: var(--ink-2);
  }
  .seg button:last-child {
    border-right: 0;
  }
  .seg button:hover {
    background: var(--brass-wash);
  }
  .seg button.on {
    background: var(--ink);
    color: var(--paper);
  }
</style>
