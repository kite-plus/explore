import {
  AppWindow,
  BookOpen,
  Braces,
  Brain,
  Briefcase,
  Camera,
  Cloud,
  Coffee,
  Database,
  Hash,
  Palette,
  Rocket,
  Server,
  ShieldCheck,
  Smartphone,
  Wrench,
  type LucideIcon,
} from "lucide-react";

// One icon for each tag in internal/model/tags.go; a tag added there
// without one here gets a plain hash.
const icons: Record<string, LucideIcon> = {
  frontend: AppWindow,
  backend: Server,
  mobile: Smartphone,
  ai: Brain,
  data: Database,
  ops: Cloud,
  security: ShieldCheck,
  languages: Braces,
  tools: Wrench,
  design: Palette,
  product: Rocket,
  career: Briefcase,
  life: Coffee,
  reading: BookOpen,
  travel: Camera,
};

export function TagIcon({ slug, className }: { slug: string; className?: string }) {
  const Icon = icons[slug] ?? Hash;
  return <Icon className={className} aria-hidden="true" />;
}
