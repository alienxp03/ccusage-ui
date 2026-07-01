import {Dialog as DialogPrimitive} from "radix-ui";
import {X} from "lucide-react";
import {cn} from "../../lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogPortal = DialogPrimitive.Portal;
export const DialogClose = DialogPrimitive.Close;
export const DialogTitle = DialogPrimitive.Title;
export const DialogDescription = DialogPrimitive.Description;

export function DialogContent({className, children, ...props}: DialogPrimitive.DialogContentProps) {
  return (
    <DialogPortal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/70" />
      <DialogPrimitive.Content
        className={cn("fixed left-1/2 top-1/2 z-50 max-h-[90vh] w-[90vw] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-app-line bg-app-bg p-4 shadow-xl", className)}
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-3 top-3 rounded-md p-1 text-app-muted hover:text-app-text">
          <X size={16} />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPortal>
  );
}
