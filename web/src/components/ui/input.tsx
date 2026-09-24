import * as React from "react";

import { cn } from "@/lib/utils";

function Input({ className, type, variant = "default", ...props }: React.ComponentProps<"input"> & { variant?: "default" | "soft" }) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-input bg-transparent px-3 py-1 text-base shadow-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive md:text-sm",
        variant === "soft" && "rounded-lg bg-muted/35 shadow-none transition-[background-color,border-color,box-shadow] hover:border-ring/60 focus-visible:bg-background focus-visible:ring-2 focus-visible:ring-ring/25 motion-reduce:transition-none",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
