import type { FC, ReactNode } from "react"

interface DashboardShellProps {
  children: ReactNode
}

export const DashboardShell: FC<DashboardShellProps> = ({ children }) => {
  return (
    <div className="flex min-h-screen w-full flex-col">
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-8 md:p-8">
        <div className="mx-auto grid w-full max-w-7xl gap-4 md:gap-8">{children}</div>
      </div>
    </div>
  )
}