import { cva } from "class-variance-authority"

export const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary text-primary-foreground",
        secondary: "border-transparent bg-secondary text-secondary-foreground",
        destructive: "border-transparent bg-destructive text-destructive-foreground",
        outline: "text-foreground",
        critical: "border-transparent bg-red-500/15 text-red-700 dark:text-red-400",
        high: "border-transparent bg-orange-500/15 text-orange-700 dark:text-orange-400",
        medium: "border-transparent bg-amber-500/15 text-amber-700 dark:text-amber-400",
        low: "border-transparent bg-blue-500/15 text-blue-700 dark:text-blue-400",
        info: "border-transparent bg-gray-500/15 text-gray-700 dark:text-gray-400",
        success: "border-transparent bg-green-500/15 text-green-700 dark:text-green-400",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)
