import { useEffect, useId, useRef, useState, type ReactNode } from "react";

interface Props {
  label: string;
  emptyLabel: string;
  plural: string;
  options: readonly string[];
  selected: readonly string[];
  onChange: (selected: string[]) => void;
  renderMarker?: (option: string) => ReactNode;
}

// Native checkboxes retain normal keyboard behavior; this is not an ARIA menu.
export function FilterDropdown({
  label,
  emptyLabel,
  plural,
  options,
  selected,
  onChange,
  renderMarker,
}: Props) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const pointerActive = useRef(false);
  const typeahead = useRef({ text: "", time: 0 });
  const id = useId();
  useEffect(() => {
    typeahead.current = { text: "", time: 0 };
    if (!open) return;
    let releaseTimer: ReturnType<typeof setTimeout> | undefined;
    const beginPointer = () => {
      pointerActive.current = true;
    };
    const endPointer = () => {
      // Click follows pointerup. Keep layout stable until the target activates.
      releaseTimer = setTimeout(() => {
        pointerActive.current = false;
        if (!rootRef.current?.contains(document.activeElement)) setOpen(false);
      }, 0);
    };
    const closeOutside = (event: MouseEvent) => {
      pointerActive.current = false;
      if (
        event.target instanceof Node &&
        !rootRef.current?.contains(event.target)
      )
        setOpen(false);
    };
    document.addEventListener("pointerdown", beginPointer, true);
    document.addEventListener("pointerup", endPointer);
    document.addEventListener("pointercancel", endPointer);
    document.addEventListener("click", closeOutside);
    return () => {
      clearTimeout(releaseTimer);
      pointerActive.current = false;
      document.removeEventListener("pointerdown", beginPointer, true);
      document.removeEventListener("pointerup", endPointer);
      document.removeEventListener("pointercancel", endPointer);
      document.removeEventListener("click", closeOutside);
    };
  }, [open]);
  const summary =
    selected.length === 0
      ? emptyLabel
      : selected.length === 1
        ? selected[0]
        : `${selected.length} ${plural} selected`;
  return (
    <div
      className="filter-dropdown"
      ref={rootRef}
      onBlur={(event) => {
        if (
          !pointerActive.current &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.preventDefault();
          event.stopPropagation();
          setOpen(false);
          buttonRef.current?.focus();
          return;
        }
        if (
          !open ||
          event.ctrlKey ||
          event.metaKey ||
          event.altKey ||
          event.nativeEvent.isComposing ||
          event.key.length !== 1 ||
          event.key === " "
        )
          return;
        const now = performance.now();
        const key = event.key.toLocaleLowerCase();
        const previous =
          now - typeahead.current.time < 750 ? typeahead.current.text : "";
        const repeated =
          previous.length > 0 &&
          [...previous].every((letter) => letter === key);
        const query = repeated ? key : previous + key;
        typeahead.current = { text: query, time: now };
        const inputs = Array.from(
          event.currentTarget.querySelectorAll<HTMLInputElement>(
            'input[type="checkbox"]',
          ),
        );
        const names = ["All", ...options];
        const current = inputs.indexOf(
          document.activeElement as HTMLInputElement,
        );
        const start = repeated ? current + 1 : 0;
        for (let offset = 0; offset < inputs.length; offset++) {
          const index = (start + offset) % inputs.length;
          if (names[index]?.toLocaleLowerCase().startsWith(query)) {
            event.preventDefault();
            inputs[index].focus({ preventScroll: true });
            inputs[index].scrollIntoView({
              block: "nearest",
              inline: "nearest",
            });
            break;
          }
        }
      }}
    >
      <span id={`${id}-label`}>{label}</span>
      <button
        ref={buttonRef}
        className="filter-trigger"
        type="button"
        aria-label={`${label}: ${summary}`}
        aria-expanded={open}
        aria-controls={`${id}-options`}
        onClick={() => setOpen((value) => !value)}
      >
        {summary}
      </button>
      {open && (
        <fieldset
          id={`${id}-options`}
          className="filter-options"
          aria-labelledby={`${id}-label`}
          onMouseDown={(event) => {
            if (event.button !== 0 || !(event.target instanceof Element))
              return;
            const input = event.target.closest("label")?.querySelector("input");
            if (!input || event.target === input) return;
            // Keep focus inside until the label's native click toggles its input.
            // Otherwise mousedown focuses the surrounding dialog and blur removes
            // the label before click. Do not intercept pointerdown/touch scrolling.
            event.preventDefault();
            input.focus({ preventScroll: true });
          }}
        >
          <label>
            <input
              type="checkbox"
              checked={selected.length === 0}
              onChange={() => onChange([])}
            />
            All
          </label>
          {options.map((option) => (
            <label key={option}>
              <input
                type="checkbox"
                checked={selected.includes(option)}
                onChange={(event) =>
                  onChange(
                    event.target.checked
                      ? [...selected, option]
                      : selected.filter((value) => value !== option),
                  )
                }
              />
              {renderMarker?.(option)}
              {option}
            </label>
          ))}
        </fieldset>
      )}
    </div>
  );
}
