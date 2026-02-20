import React, { useState, useEffect, useCallback } from 'react';
import { LayoutGrid, CheckCircle2, RefreshCw, Settings, MessageCircle, Menu, ChevronsLeft, ChevronsRight, Search, X, Film, ArrowDownToLine, BookOpen, Wrench, Globe } from 'lucide-react';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { Badge } from './components/ui/badge';
import AppList from './components/AppList';
import AppDetailDialog from './components/AppDetailDialog';
import ProgressOverlay from './components/ProgressOverlay';
import SettingsDialog from './components/SettingsDialog';
import { fetchApps, triggerCheck, installApp, updateApp, uninstallApp, fetchStatus, triggerStoreUpdate } from './api/client';
import type { AppInfo, AppOperation, SSECallback } from './api/client';
import { toast } from "sonner"
import { Toaster } from "@/components/ui/sonner"
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet"
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"

const CATEGORIES = [
  { key: 'media', label: '媒体服务', icon: Film },
  { key: 'download', label: '下载传输', icon: ArrowDownToLine },
  { key: 'content', label: '内容管理', icon: BookOpen },
  { key: 'system', label: '系统工具', icon: Wrench },
  { key: 'browser', label: '浏览器', icon: Globe },
] as const;

type CategoryKey = typeof CATEGORIES[number]['key'];

const App: React.FC = () => {
  const [apps, setApps] = useState<AppInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [checking, setChecking] = useState<boolean>(false);
  const [lastCheck, setLastCheck] = useState<string>('');

  const [appOperations, setAppOperations] = useState<Map<string, AppOperation>>(new Map());
  const [selfUpdateActive, setSelfUpdateActive] = useState(false);
  const [selfUpdateState, setSelfUpdateState] = useState<{message: string; progress: number; speed?: number; downloaded?: number; total?: number} | null>(null);

  const [settingsVisible, setSettingsVisible] = useState(false);
  const [activeFilter, setActiveFilter] = useState<'all' | 'installed' | 'update_available'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState<CategoryKey | null>(null);
  const [pendingUninstallApp, setPendingUninstallApp] = useState<AppInfo | null>(null);
  const [detailApp, setDetailApp] = useState<AppInfo | null>(null);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() =>
    localStorage.getItem('sidebar-collapsed') === 'true'
  );

  const toggleSidebar = useCallback(() => {
    setSidebarCollapsed(prev => {
      const next = !prev;
      localStorage.setItem('sidebar-collapsed', String(next));
      return next;
    });
  }, []);

  useEffect(() => {
    loadApps();
  }, []);

  const setAppOp = useCallback((appname: string, op: AppOperation | null) => {
    setAppOperations(prev => {
      const next = new Map(prev);
      if (op === null) {
        next.delete(appname);
      } else {
        next.set(appname, op);
      }
      return next;
    });
  }, []);

  const pollForRestart = () => {
    let retries = 0;
    const poll = async () => {
      try {
        await fetchStatus();
        window.location.reload();
      } catch {
        retries++;
        if (retries > 30) {
          setSelfUpdateState({ message: '重启超时，请手动刷新页面', progress: 100 });
          return;
        }
        setSelfUpdateState({ message: '正在重启...', progress: 100 });
        setTimeout(poll, 2000);
      }
    };
    setTimeout(poll, 2000);
  };

  const translateStep = (step?: string) => {
      switch(step) {
          case 'downloading': return '正在下载...';
          case 'installing': return '正在安装...';
          case 'verifying': return '正在验证...';
          case 'starting': return '正在启动...';
          case 'uninstalling': return '正在卸载...';
          default: return '处理中...';
      }
  };

  const loadApps = async () => {
    try {
      const data = await fetchApps();
      setApps(data.apps);
      setLastCheck(data.last_check);
    } catch (error) {
      console.error('Failed to load apps:', error);
    } finally {
      setLoading(false);
    }
  };

  const createSSEHandler = useCallback((appname: string): SSECallback => (data) => {
    if (data.step === 'self_update') {
      setSelfUpdateActive(true);
      setSelfUpdateState({ message: data.message || '商店正在更新，请稍候...', progress: 100 });
      pollForRestart();
      return;
    }

    if (data.step === 'error') {
      toast.error(data.message || '发生未知错误');
      setAppOp(appname, null);
      loadApps();
      return;
    }

    if (data.step === 'done') {
      setAppOp(appname, null);
      loadApps();
      return;
    }

    setAppOp(appname, {
      step: data.step || 'processing',
      progress: data.progress || 0,
      message: data.message || translateStep(data.step),
      speed: data.speed,
      downloaded: data.downloaded,
      total: data.total,
    });
  }, [setAppOp]);

  const handleCheck = async () => {
    setChecking(true);
    try {
      await triggerCheck();
      await loadApps();
    } catch (error) {
      console.error('Check failed:', error);
      toast.error('检查更新失败');
    } finally {
      setChecking(false);
    }
  };

  const handleInstall = useCallback(async (app: AppInfo) => {
    const appname = app.appname;
    const handler = createSSEHandler(appname);

    setAppOp(appname, {
      step: 'starting',
      progress: 0,
      message: `正在安装 ${app.display_name}...`,
    });

    const handle = installApp(appname, handler);
    setAppOp(appname, {
      step: 'starting',
      progress: 0,
      message: `正在安装 ${app.display_name}...`,
      cancel: handle.cancel,
    });

    try {
      await handle.promise;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        toast.info('已取消');
        setAppOp(appname, null);
        return;
      }
      console.error(error);
      toast.error('安装请求失败');
    } finally {
      setAppOperations(prev => {
        if (prev.has(appname)) {
          const next = new Map(prev);
          next.delete(appname);
          loadApps();
          return next;
        }
        return prev;
      });
    }
  }, [createSSEHandler, setAppOp]);

  const handleUpdate = useCallback(async (app: AppInfo) => {
    const appname = app.appname;
    const handler = createSSEHandler(appname);

    setAppOp(appname, {
      step: 'starting',
      progress: 0,
      message: `正在更新 ${app.display_name}...`,
    });

    const handle = updateApp(appname, handler);
    setAppOp(appname, {
      step: 'starting',
      progress: 0,
      message: `正在更新 ${app.display_name}...`,
      cancel: handle.cancel,
    });

    try {
      await handle.promise;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        toast.info('已取消');
        setAppOp(appname, null);
        return;
      }
      console.error(error);
      toast.error('更新请求失败');
    } finally {
      setAppOperations(prev => {
        if (prev.has(appname)) {
          const next = new Map(prev);
          next.delete(appname);
          loadApps();
          return next;
        }
        return prev;
      });
    }
  }, [createSSEHandler, setAppOp]);

  const handleUninstall = (app: AppInfo) => {
    setPendingUninstallApp(app);
  };

  const confirmUninstall = useCallback(async () => {
    if (!pendingUninstallApp) return;
    const app = pendingUninstallApp;
    setPendingUninstallApp(null);

    const appname = app.appname;
    const handler = createSSEHandler(appname);

    setAppOp(appname, {
      step: 'uninstalling',
      progress: 0,
      message: `正在卸载 ${app.display_name}...`,
    });

    const handle = uninstallApp(appname, handler);

    try {
      await handle.promise;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        toast.info('已取消');
        setAppOp(appname, null);
        return;
      }
      console.error(error);
      toast.error('卸载请求失败');
    } finally {
      setAppOperations(prev => {
        if (prev.has(appname)) {
          const next = new Map(prev);
          next.delete(appname);
          loadApps();
          return next;
        }
        return prev;
      });
    }
  }, [pendingUninstallApp, createSSEHandler, setAppOp]);

  const handleCancelOp = useCallback((app: AppInfo) => {
    const op = appOperations.get(app.appname);
    if (op?.cancel) {
      op.cancel();
      toast.info('已取消');
      setAppOp(app.appname, null);
      loadApps();
    }
  }, [appOperations, setAppOp]);

  const handleStoreUpdate = useCallback(async () => {
    setSelfUpdateActive(true);
    setSelfUpdateState({ message: '正在更新商店...', progress: 0 });

    const handle = triggerStoreUpdate((data) => {
      if (data.step === 'self_update') {
        setSelfUpdateState({ message: '商店正在重启...', progress: 100 });
        pollForRestart();
        return;
      }
      if (data.step === 'error') {
        toast.error(data.message || '商店更新失败');
        setSelfUpdateActive(false);
        setSelfUpdateState(null);
        return;
      }
      setSelfUpdateState({
        message: data.message || '正在更新商店...',
        progress: data.progress || 0,
        speed: data.speed,
        downloaded: data.downloaded,
        total: data.total,
      });
    });

    try {
      await handle.promise;
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      console.error(error);
      toast.error('商店更新失败');
      setSelfUpdateActive(false);
      setSelfUpdateState(null);
    }
  }, []);

  const filteredApps = apps.filter(app => {
    if (activeFilter === 'installed' && !app.installed) return false;
    if (activeFilter === 'update_available' && !app.has_update) return false;
    if (activeCategory && app.category !== activeCategory) return false;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      const name = (app.display_name || '').toLowerCase();
      const appname = (app.appname || '').toLowerCase();
      const desc = (app.description || '').toLowerCase();
      if (!name.includes(q) && !appname.includes(q) && !desc.includes(q)) return false;
    }

    return true;
  });

  const counts = {
      all: apps.length,
      installed: apps.filter(a => a.installed).length,
      update_available: apps.filter(a => a.has_update).length
  };

  const categoryCounts = CATEGORIES.reduce((acc, cat) => {
    acc[cat.key] = apps.filter(a => a.category === cat.key).length;
    return acc;
  }, {} as Record<string, number>);

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col md:flex-row">
      <aside className={cn(
        "hidden md:flex flex-col bg-card border-r border-border h-screen sticky top-0 transition-all duration-300 overflow-hidden",
        sidebarCollapsed ? "w-[68px]" : "w-64"
      )}>
        <TooltipProvider delayDuration={0}>
         <div className={cn("border-b border-border shrink-0", sidebarCollapsed ? "p-3 flex items-center justify-center" : "p-6")}>
           {sidebarCollapsed ? (
             <Button variant="ghost" size="icon" className="h-8 w-8" onClick={toggleSidebar}>
               <ChevronsRight className="h-4 w-4" />
             </Button>
           ) : (
             <div className="flex items-start justify-between gap-2">
               <div className="min-w-0">
                 <h1 className="text-xl font-semibold tracking-tight whitespace-nowrap">fnOS Apps</h1>
                 <p className="text-sm text-muted-foreground mt-1.5 whitespace-nowrap">
                    上次检查: {lastCheck ? new Date(lastCheck).toLocaleString() : '从未'}
                 </p>
               </div>
               <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 -mr-2 -mt-1" onClick={toggleSidebar}>
                 <ChevronsLeft className="h-4 w-4" />
               </Button>
             </div>
           )}
         </div>

         <div className="flex-1 overflow-y-auto">
          <nav className={cn("space-y-1", sidebarCollapsed ? "p-2" : "p-4")}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant={activeFilter === 'all' ? 'secondary' : 'ghost'}
                  className={cn("w-full h-10 shadow-none", sidebarCollapsed ? "justify-center px-0" : "justify-start px-3")}
                  onClick={() => setActiveFilter('all')}
                >
                  <LayoutGrid className={cn("h-4 w-4 shrink-0", !sidebarCollapsed && "mr-3")} />
                  {!sidebarCollapsed && (
                    <>
                      <span className="flex-1 text-left whitespace-nowrap">全部</span>
                      <span className="ml-auto text-xs text-muted-foreground tabular-nums">{counts.all}</span>
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              {sidebarCollapsed && <TooltipContent side="right">全部 ({counts.all})</TooltipContent>}
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant={activeFilter === 'installed' ? 'secondary' : 'ghost'}
                  className={cn("w-full h-10 shadow-none", sidebarCollapsed ? "justify-center px-0" : "justify-start px-3")}
                  onClick={() => setActiveFilter('installed')}
                >
                  <CheckCircle2 className={cn("h-4 w-4 shrink-0", !sidebarCollapsed && "mr-3")} />
                  {!sidebarCollapsed && (
                    <>
                      <span className="flex-1 text-left whitespace-nowrap">已安装</span>
                      <span className="ml-auto text-xs text-muted-foreground tabular-nums">{counts.installed}</span>
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              {sidebarCollapsed && <TooltipContent side="right">已安装 ({counts.installed})</TooltipContent>}
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant={activeFilter === 'update_available' ? 'secondary' : 'ghost'}
                  className={cn("w-full h-10 shadow-none", sidebarCollapsed ? "justify-center px-0" : "justify-start px-3")}
                  onClick={() => setActiveFilter('update_available')}
                >
                  <div className="relative shrink-0">
                    <RefreshCw className={cn("h-4 w-4", !sidebarCollapsed && "mr-3")} />
                    {sidebarCollapsed && counts.update_available > 0 && (
                      <span className="absolute -top-1 -right-1 h-2 w-2 rounded-full bg-destructive" />
                    )}
                  </div>
                  {!sidebarCollapsed && (
                    <>
                      <span className="flex-1 text-left whitespace-nowrap">有更新</span>
                      {counts.update_available > 0 ? (
                        <Badge variant="destructive" className="ml-auto shrink-0">{counts.update_available}</Badge>
                      ) : (
                        <span className="ml-auto text-xs text-muted-foreground tabular-nums">0</span>
                      )}
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              {sidebarCollapsed && (
                <TooltipContent side="right">有更新 ({counts.update_available})</TooltipContent>
              )}
            </Tooltip>
          </nav>

          <div className={cn("border-t border-border", sidebarCollapsed ? "p-2 pt-3" : "px-4 pb-4 pt-3")}>
            {!sidebarCollapsed && (
              <p className="text-xs font-medium text-muted-foreground mb-2 px-3">分类</p>
            )}
            <div className="space-y-1">
              {CATEGORIES.map(cat => {
                const Icon = cat.icon;
                const isActive = activeCategory === cat.key;
                const count = categoryCounts[cat.key];
                return (
                  <Tooltip key={cat.key}>
                    <TooltipTrigger asChild>
                      <Button
                        variant={isActive ? 'secondary' : 'ghost'}
                        className={cn("w-full h-10 shadow-none", sidebarCollapsed ? "justify-center px-0" : "justify-start px-3")}
                        onClick={() => setActiveCategory(isActive ? null : cat.key)}
                      >
                        <Icon className={cn("h-4 w-4 shrink-0", !sidebarCollapsed && "mr-3")} />
                        {!sidebarCollapsed && (
                          <>
                            <span className="flex-1 text-left whitespace-nowrap">{cat.label}</span>
                            <span className="ml-auto text-xs text-muted-foreground tabular-nums">{count}</span>
                          </>
                        )}
                      </Button>
                    </TooltipTrigger>
                    {sidebarCollapsed && <TooltipContent side="right">{cat.label} ({count})</TooltipContent>}
                  </Tooltip>
                );
              })}
            </div>
          </div>
         </div>

         <div className={cn("mt-auto border-t border-border space-y-1", sidebarCollapsed ? "p-2" : "p-4")}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  className={cn(
                    "w-full h-10 shadow-none text-muted-foreground hover:text-foreground",
                    sidebarCollapsed ? "justify-center px-0" : "justify-start px-3"
                  )}
                  onClick={() => window.open('https://github.com/conversun/fnos-apps/issues', '_blank')}
                >
                  <MessageCircle className={cn("h-4 w-4 shrink-0", !sidebarCollapsed && "mr-3")} />
                  {!sidebarCollapsed && <span className="flex-1 text-left whitespace-nowrap">问题反馈</span>}
                </Button>
              </TooltipTrigger>
              {sidebarCollapsed && <TooltipContent side="right">问题反馈</TooltipContent>}
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  className={cn(
                    "w-full h-10 shadow-none text-muted-foreground hover:text-foreground",
                    sidebarCollapsed ? "justify-center px-0" : "justify-start px-3"
                  )}
                  onClick={() => setSettingsVisible(true)}
                >
                  <Settings className={cn("h-4 w-4 shrink-0", !sidebarCollapsed && "mr-3")} />
                  {!sidebarCollapsed && <span className="flex-1 text-left whitespace-nowrap">设置</span>}
                </Button>
              </TooltipTrigger>
              {sidebarCollapsed && <TooltipContent side="right">设置</TooltipContent>}
            </Tooltip>
         </div>
        </TooltipProvider>
       </aside>

      <div className="flex-1 flex flex-col min-h-screen">
        <div className="md:hidden bg-card border-b border-border p-4 sticky top-0 z-20 flex flex-col gap-3">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Sheet>
                        <SheetTrigger asChild>
                            <Button variant="ghost" size="icon">
                                <Menu className="h-5 w-5" />
                            </Button>
                        </SheetTrigger>
                        <SheetContent side="left" className="w-64 p-0">
                             <div className="p-6 border-b border-border">
                                <h1 className="text-xl font-semibold tracking-tight">fnOS Apps</h1>
                                <p className="text-sm text-muted-foreground mt-1.5">
                                   上次检查: {lastCheck ? new Date(lastCheck).toLocaleString() : '从未'}
                                </p>
                             </div>
                             <nav className="flex-1 p-4 space-y-1">
                                 <Button
                                   variant={activeFilter === 'all' ? 'secondary' : 'ghost'}
                                   className="w-full justify-start h-10 px-3 shadow-none"
                                   onClick={() => setActiveFilter('all')}
                                 >
                                    <LayoutGrid className="mr-3 h-4 w-4 shrink-0" />
                                    <span className="flex-1 text-left">全部</span>
                                    <span className="ml-auto text-xs text-muted-foreground tabular-nums">{counts.all}</span>
                                 </Button>
                                 <Button
                                   variant={activeFilter === 'installed' ? 'secondary' : 'ghost'}
                                   className="w-full justify-start h-10 px-3 shadow-none"
                                   onClick={() => setActiveFilter('installed')}
                                 >
                                    <CheckCircle2 className="mr-3 h-4 w-4 shrink-0" />
                                    <span className="flex-1 text-left">已安装</span>
                                    <span className="ml-auto text-xs text-muted-foreground tabular-nums">{counts.installed}</span>
                                 </Button>
                                 <Button
                                   variant={activeFilter === 'update_available' ? 'secondary' : 'ghost'}
                                   className="w-full justify-start h-10 px-3 shadow-none"
                                   onClick={() => setActiveFilter('update_available')}
                                 >
                                    <RefreshCw className="mr-3 h-4 w-4 shrink-0" />
                                    <span className="flex-1 text-left">有更新</span>
                                    {counts.update_available > 0 ? (
                                      <Badge variant="destructive" className="ml-auto shrink-0">{counts.update_available}</Badge>
                                    ) : (
                                      <span className="ml-auto text-xs text-muted-foreground tabular-nums">0</span>
                                    )}
                                 </Button>
                              </nav>
                              <div className="px-4 pt-3 border-t border-border">
                                <p className="text-xs font-medium text-muted-foreground mb-2 px-3">分类</p>
                                <div className="space-y-1">
                                  {CATEGORIES.map(cat => {
                                    const Icon = cat.icon;
                                    const isActive = activeCategory === cat.key;
                                    const count = categoryCounts[cat.key];
                                    return (
                                      <Button
                                        key={cat.key}
                                        variant={isActive ? 'secondary' : 'ghost'}
                                        className="w-full justify-start h-10 px-3 shadow-none"
                                        onClick={() => setActiveCategory(isActive ? null : cat.key)}
                                      >
                                        <Icon className="mr-3 h-4 w-4 shrink-0" />
                                        <span className="flex-1 text-left">{cat.label}</span>
                                        <span className="ml-auto text-xs text-muted-foreground tabular-nums">{count}</span>
                                      </Button>
                                    );
                                  })}
                                </div>
                              </div>
                              <div className="p-4 mt-auto border-t border-border space-y-1">
                                 <Button
                                   variant="ghost"
                                   className="w-full justify-start h-10 px-3 shadow-none text-muted-foreground hover:text-foreground"
                                   onClick={() => window.open('https://github.com/conversun/fnos-apps/issues', '_blank')}
                                 >
                                    <MessageCircle className="mr-3 h-4 w-4 shrink-0" />
                                    <span className="flex-1 text-left">问题反馈</span>
                                 </Button>
                                 <Button
                                   variant="ghost"
                                   className="w-full justify-start h-10 px-3 shadow-none text-muted-foreground hover:text-foreground"
                                   onClick={() => setSettingsVisible(true)}
                                 >
                                    <Settings className="mr-3 h-4 w-4 shrink-0" />
                                    <span className="flex-1 text-left">设置</span>
                                 </Button>
                              </div>
                        </SheetContent>
                    </Sheet>
                    <h1 className="text-xl font-bold">fnOS Apps</h1>
                </div>
            </div>
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
              <Input
                type="text"
                placeholder="搜索应用..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-8 pr-8 h-9 shadow-none"
              />
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery('')}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>
        </div>

        <header className="hidden md:flex bg-card border-b border-border px-8 py-4 justify-between items-center sticky top-0 z-10">
           <h2 className="text-lg font-medium shrink-0">
              {activeFilter === 'all' && '全部应用'}
              {activeFilter === 'installed' && '已安装应用'}
              {activeFilter === 'update_available' && '可用更新'}
              {activeCategory && (
                <span className="text-muted-foreground font-normal">{' · '}{CATEGORIES.find(c => c.key === activeCategory)?.label}</span>
              )}
           </h2>
           <div className="flex items-center gap-3">
               <div className="relative">
                 <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
                 <Input
                   type="text"
                   placeholder="搜索应用..."
                   value={searchQuery}
                   onChange={(e) => setSearchQuery(e.target.value)}
                    className="w-56 pl-8 pr-8 h-9 shadow-none"
                 />
                 {searchQuery && (
                   <button
                     onClick={() => setSearchQuery('')}
                     className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                   >
                     <X className="h-4 w-4" />
                   </button>
                 )}
               </div>
               <Button 
                 onClick={handleCheck} 
                 disabled={checking}
               >
                 {checking ? (
                   <>
                     <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                     检查中...
                   </>
                 ) : (
                   <>
                     <RefreshCw className="mr-2 h-4 w-4" />
                     立即检查
                   </>
                 )}
               </Button>
           </div>
        </header>

        <main className="flex-grow p-4 md:p-8 overflow-y-auto">
          <AppList
             apps={filteredApps}
             loading={loading}
             onInstall={handleInstall}
             onUpdate={handleUpdate}
             onUninstall={handleUninstall}
             onDetail={setDetailApp}
             onCancelOp={handleCancelOp}
             filterType={activeFilter}
             appOperations={appOperations}
             searchQuery={searchQuery}
           />
        </main>
      </div>

      {selfUpdateActive && selfUpdateState && (
        <ProgressOverlay
          visible={true}
          message={selfUpdateState.message}
          progress={selfUpdateState.progress}
          speed={selfUpdateState.speed}
          downloaded={selfUpdateState.downloaded}
          total={selfUpdateState.total}
        />
      )}
      
      {settingsVisible && (
        <SettingsDialog
            visible={settingsVisible}
            onClose={() => setSettingsVisible(false)}
            onStoreUpdate={handleStoreUpdate}
        />
      )}
      
      <AlertDialog open={!!pendingUninstallApp} onOpenChange={(open) => !open && setPendingUninstallApp(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认卸载</AlertDialogTitle>
            <AlertDialogDescription>
              确定要卸载 {pendingUninstallApp?.display_name} 吗？此操作无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={confirmUninstall}>
              确认卸载
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AppDetailDialog
        app={detailApp}
        open={!!detailApp}
        onOpenChange={(open) => !open && setDetailApp(null)}
        onInstall={handleInstall}
        onUpdate={handleUpdate}
        operation={detailApp ? appOperations.get(detailApp.appname) : undefined}
      />

      <Toaster />
    </div>
  );
};

export default App;
