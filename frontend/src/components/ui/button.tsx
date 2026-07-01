import {forwardRef, type ButtonHTMLAttributes} from "react";
import {cn} from "../../lib/utils";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "default" | "ghost" | "outline";
  size?: "default" | "icon" | "sm";
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(({className, variant = "default", size = "default", ...props}, ref) => (
  <button
    ref={ref}
    className={cn(
      "inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium transition disabled:pointer-events-none disabled:opacity-50",
      variant === "default" && "bg-app-accent text-white hover:bg-app-accent/90",
      variant === "ghost" && "hover:bg-app-surface",
      variant === "outline" && "border border-app-line bg-app-surface hover:bg-app-panel",
      size === "default" && "h-9 px-3",
      size === "sm" && "h-8 px-2.5 text-xs",
      size === "icon" && "h-9 w-9",
      className,
    )}
    {...props}
  />
));
Button.displayName = "Button";
