'use client'

import Link from "next/link"
import { Cloud, Bell, Settings, HelpCircle, ChevronDown, Cpu, HardDrive, Database, Network } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import * as DropdownMenu from "@radix-ui/react-dropdown-menu"

export function MainNav() {
  return (
    <div className="flex w-full items-center justify-between">
      <div className="flex items-center gap-6 md:gap-10">
        <Link href="/" className="flex items-center space-x-2">
          <Cloud className="h-6 w-6" />
          <span className="hidden font-bold sm:inline-block">OpenCloud</span>
        </Link>
        <nav className="hidden gap-6 md:flex">
          <Link href="/" className="flex items-center text-sm font-medium text-foreground">
            Dashboard
          </Link>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <button className="flex items-center gap-1 text-sm font-medium text-foreground hover:text-primary">
                Compute
                <ChevronDown className="h-4 w-4" />
              </button>
            </DropdownMenu.Trigger>

            <DropdownMenu.Portal>
              <DropdownMenu.Content
                className="min-w-[180px] rounded-md bg-white p-1 shadow-lg ring-1 ring-black/5 dark:bg-neutral-900"
                sideOffset={5}
              >
                <DropdownMenu.Item className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800">
                  <Cloud className="h-4 w-4" />
                  Functions (Coming Soon)
                </DropdownMenu.Item>
                <Link
                  href="/compute/containers"
                  className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800"
                >
                  <HardDrive className="h-4 w-4" />
                  Containers
                </Link>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <button className="flex items-center gap-1 text-sm font-medium text-foreground hover:text-primary">
                Storage
                <ChevronDown className="h-4 w-4" />
              </button>
            </DropdownMenu.Trigger>

            <DropdownMenu.Portal>
              <DropdownMenu.Content
                className="min-w-[180px] rounded-md bg-white p-1 shadow-lg ring-1 ring-black/5 dark:bg-neutral-900"
                sideOffset={5}
              >
                <DropdownMenu.Item className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800">
                  <Cloud className="h-4 w-4" />
                  Blob (Coming Soon)
                </DropdownMenu.Item>
                <DropdownMenu.Item className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800">
                  <HardDrive className="h-4 w-4" />
                  Databases (Coming Soon)
                </DropdownMenu.Item>
                <Link
                  href="/storage/containers"
                  className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800"
                >
                  <HardDrive className="h-4 w-4" />
                  Container Storage
                </Link>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <button className="flex items-center gap-1 text-sm font-medium text-foreground hover:text-primary">
                CI/CD
                <ChevronDown className="h-4 w-4" />
              </button>
            </DropdownMenu.Trigger>

            <DropdownMenu.Portal>
              <DropdownMenu.Content
                className="min-w-[180px] rounded-md bg-white p-1 shadow-lg ring-1 ring-black/5 dark:bg-neutral-900"
                sideOffset={5}
              >
                <DropdownMenu.Item className="flex cursor-pointer select-none items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground hover:bg-neutral-100 dark:hover:bg-neutral-800">
                  <Cloud className="h-4 w-4" />
                  Pipeline (Coming Soon)
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
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
