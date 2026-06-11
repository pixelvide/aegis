import { Toaster as Sonner } from "sonner"

type ToasterProps = React.ComponentProps<typeof Sonner>

function Toaster({ ...props }: ToasterProps) {
  return (
    <Sonner
      position="bottom-right"
      toastOptions={{
        style: {
          borderRadius: "0px",
        },
        classNames: {
          toast:
            "group toast border bg-background text-foreground shadow-lg",
          title: "text-sm font-semibold",
          description: "text-sm text-muted-foreground",
          actionButton:
            "bg-primary text-primary-foreground text-xs font-medium px-3 py-1.5",
          cancelButton:
            "bg-muted text-muted-foreground text-xs font-medium px-3 py-1.5",
          success: "!border-green-600/30 !bg-green-50 !text-green-900 dark:!bg-green-950 dark:!text-green-100",
          error: "!border-red-600/30 !bg-red-50 !text-red-900 dark:!bg-red-950 dark:!text-red-100",
          warning: "!border-yellow-600/30 !bg-yellow-50 !text-yellow-900 dark:!bg-yellow-950 dark:!text-yellow-100",
          info: "!border-blue-600/30 !bg-blue-50 !text-blue-900 dark:!bg-blue-950 dark:!text-blue-100",
        },
      }}
      duration={10000}
      closeButton
      richColors={false}
      {...props}
    />
  )
}

export { Toaster }
