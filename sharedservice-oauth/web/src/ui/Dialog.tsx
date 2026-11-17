import { type ReactNode, useEffect, useId, useRef } from "react";

// Dialog wraps a native modal <dialog>: the browser traps focus, makes the page
// inert and closes on Escape (the cancel event). Focus goes back to the opener.
export function Dialog(props: { title: string; onClose: () => void; children: ReactNode; description?: string }) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descriptionId = useId();
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
    dialog.showModal();
    return () => {
      if (dialog.open) dialog.close();
      opener?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="dialog"
      aria-labelledby={titleId}
      aria-describedby={props.description ? descriptionId : undefined}
      onCancel={(event) => {
        event.preventDefault();
        props.onClose();
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) props.onClose();
      }}
    >
      <div className="dialog-body">
        <h2 id={titleId}>{props.title}</h2>
        {props.description && (
          <p id={descriptionId} className="muted">
            {props.description}
          </p>
        )}
        {props.children}
      </div>
    </dialog>
  );
}
