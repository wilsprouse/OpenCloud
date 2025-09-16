import Link from "next/link"
import { Cloud, Bell, Settings, HelpCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"

export function MainNav() {
  return (
    <div className="flex w-full items-center justify-between">
      <div className="flex items-center gap-6 md:gap-10">
        <Link href="/" className="flex items-center space-x-2">
          <Cloud className="h-6 w-6" />
          <span className="hidden font-bold sm:inline-block">CloudOS</span>
        </Link>
        <nav className="hidden gap-6 md:flex">
          <Link href="/" className="flex items-center text-sm font-medium text-foreground">
            Dashboard
          </Link>
          <Link href="#" className="flex items-center text-sm font-medium text-muted-foreground">
            Compute
          </Link>
          <Link href="#" className="flex items-center text-sm font-medium text-muted-foreground">
            Storage
          </Link>
          <Link href="#" className="flex items-center text-sm font-medium text-muted-foreground">
            Databases
          </Link>
          <Link href="#" className="flex items-center text-sm font-medium text-muted-foreground">
            Networking
          </Link>
        </nav>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon">
          <Bell className="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon">
          <HelpCircle className="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon">
          <Settings className="h-5 w-5" />
        </Button>
        <Avatar>
          <AvatarImage src="/placeholder-user.jpg" alt="User" />
          <AvatarFallback>U</AvatarFallback>
        </Avatar>
      </div>
    </div>
  )
}
