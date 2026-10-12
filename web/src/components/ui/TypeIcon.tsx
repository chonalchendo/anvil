import {
  BookMarked, Boxes, CircleDot, Compass, FolderKanban, Hash, Inbox, Lightbulb, MessagesSquare,
  Milestone, Puzzle, Scale, type LucideIcon,
} from "lucide-react";
import { cn } from "../../lib/cn";

const icons: Record<string, LucideIcon> = {
  project: FolderKanban, milestone: Milestone, issue: CircleDot, "product-design": Compass,
  "system-design": Boxes, "component-design": Puzzle, decision: Scale, learning: Lightbulb,
  thread: MessagesSquare, inbox: Inbox, convention: BookMarked, topic: Hash,
};

export function TypeIcon({ type, className }: { type: string; className?: string }) {
  const Icon = icons[type] ?? CircleDot;
  return <Icon size={14} strokeWidth={1.75} className={cn("shrink-0 text-faint", className)} aria-hidden />;
}
