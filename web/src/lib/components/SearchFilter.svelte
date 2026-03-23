<script lang="ts">
  interface Props {
    value: string;
    placeholder?: string;
    onchange: (value: string) => void;
  }

  let { value, placeholder = 'Filter specs\u2026', onchange }: Props = $props();
</script>

<div class="search-filter">
  <input
    type="text"
    {placeholder}
    {value}
    oninput={(e) => onchange(e.currentTarget.value)}
  />
</div>

<style>
  .search-filter {
    display: flex;
  }

  input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
    font-size: 0.9rem;
    color: #1a1a2e;
    background: white;
    outline: none;
    transition: border-color 0.15s;
  }

  input:focus {
    border-color: #2563eb;
    box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
  }

  input::placeholder {
    color: #94a3b8;
  }
</style>
