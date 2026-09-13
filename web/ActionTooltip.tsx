import {
  cloneElement,
  useEffect,
  useId,
  useRef,
  useState,
  type ReactElement,
} from "react";

const hoverDelayMs = 300;

export function ActionTooltip({
  label,
  children,
  inline = false,
  help = false,
}: {
  label: string;
  children: ReactElement<{ "aria-describedby"?: string }>;
  inline?: boolean;
  help?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const focused = useRef(false);
  const hovered = useRef(false);
  const root = useRef<HTMLSpanElement>(null);
  const clear = () => {
    clearTimeout(timer.current);
    timer.current = undefined;
  };
  useEffect(() => () => clearTimeout(timer.current), []);
  useEffect(() => {
    if (!open) return;
    const dismiss = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      clear();
      setOpen(false);
    };
    document.addEventListener("keydown", dismiss, true);
    const outside = (event: PointerEvent) => {
      if (
        help &&
        event.target instanceof Node &&
        !root.current?.contains(event.target)
      ) {
        clear();
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", outside);
    return () => {
      document.removeEventListener("keydown", dismiss, true);
      document.removeEventListener("pointerdown", outside);
    };
  }, [open, help]);
  return (
    <span
      ref={root}
      className={
        help
          ? "admission-help"
          : inline
            ? "inline-action-tooltip"
            : "event-action-slot"
      }
      onClick={
        help
          ? () => {
              clear();
              setOpen(true);
            }
          : undefined
      }
      onPointerEnter={(event) => {
        if (event.pointerType !== "mouse") return;
        hovered.current = true;
        clear();
        if (!focused.current)
          timer.current = setTimeout(() => setOpen(true), hoverDelayMs);
      }}
      onPointerLeave={() => {
        hovered.current = false;
        clear();
        if (!focused.current) setOpen(false);
      }}
      onFocus={() => {
        focused.current = true;
        clear();
        setOpen(true);
      }}
      onBlur={() => {
        focused.current = false;
        clear();
        if (!hovered.current) setOpen(false);
      }}
    >
      {cloneElement(children, { "aria-describedby": open ? id : undefined })}
      {open && (
        <span className="action-tooltip" role="tooltip" id={id}>
          <span>{label}</span>
        </span>
      )}
    </span>
  );
}
