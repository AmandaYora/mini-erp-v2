// Re-export primitif shadcn yang boleh dipakai modul (PLAN U3): modul tidak mengimpor
// `primitives/*` langsung (ESLint no-restricted-imports), melainkan lewat fasad ini.
export { Checkbox } from "../primitives/checkbox";
export { RadioGroup, RadioGroupItem } from "../primitives/radio-group";
export { Switch } from "../primitives/switch";
export { Tabs, TabsList, TabsTrigger, TabsContent } from "../primitives/tabs";
export { ToggleGroup, ToggleGroupItem } from "../primitives/toggle-group";
export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "../primitives/tooltip";
export {
  Sheet,
  SheetTrigger,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetDescription,
} from "../primitives/sheet";
export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from "../primitives/dropdown-menu";
export { Popover, PopoverTrigger, PopoverContent, PopoverAnchor } from "../primitives/popover";
export {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "../primitives/breadcrumb";
export { Avatar, AvatarImage, AvatarFallback } from "../primitives/avatar";
export { Collapsible, CollapsibleTrigger, CollapsibleContent } from "../primitives/collapsible";
export {
  Command,
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandShortcut,
  CommandSeparator,
} from "../primitives/command";
export { Kbd, KbdGroup } from "../primitives/kbd";
export { Separator } from "../primitives/separator";
export { Progress } from "../primitives/progress";
export { Skeleton } from "../primitives/skeleton";
export { Spinner } from "../primitives/spinner";
export { ScrollArea } from "../primitives/scroll-area";
export { Item, ItemMedia, ItemContent, ItemActions, ItemGroup, ItemTitle, ItemDescription } from "../primitives/item";
