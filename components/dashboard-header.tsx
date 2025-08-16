import type { FC, ReactNode } from "react"

interface DashboardHeaderProps {
  heading: string
  text?: string
  children?: ReactNode
}

export const DashboardHeader: FC<DashboardHeaderProps> = ({ heading, text, children }) => {
  return (
    <div className="flex items-center justify-between px-2">
      <div className="grid gap-1">
        <h1 className="font-heading text-3xl md:text-4xl">{heading}</h1>
        {text && <p className="text-lg text-muted-foreground">{text}</p>}
      </div>
      {children}
    </div>
  )
}
