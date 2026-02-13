import React, { useState, useEffect, useRef } from 'react';
import { LayoutGrid, CheckCircle2, RefreshCw, Settings, MessageCircle, Menu } from 'lucide-react';
import { Button } from './components/ui/button';
import { Badge } from './components/ui/badge';
import AppList from './components/AppList';
import ProgressOverlay from './components/ProgressOverlay';
import SettingsDialog from './components/SettingsDialog';
import { fetchApps, triggerCheck, installApp, updateApp, uninstallApp, getSSEEventSource, fetchStatus } from './api/client';
import type { AppInfo, UpdateProgress } from './api/client';
import { toast } from "sonner"
import { Toaster } from "@/components/ui/sonner"
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet"
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

const App: React.FC = () => {
  const [apps, setApps] = useState<AppInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [checking, setChecking] = useState<boolean>(false);
  const [lastCheck, setLastCheck] = useState<string>('');
  const [isUpdatingStore, setIsUpdatingStore] = useState(false);
  
  // Progress state
  const [progressVisible, setProgressVisible] = useState(false);
  const [progressState, setProgressState] = useState<UpdateProgress>({ step: '', progress: 0, message: '' });
  const [activeApp, setActiveApp] = useState<string | null>(null);

  const [settingsVisible, setSettingsVisible] = useState(false);
  const [activeFilter, setActiveFilter] = useState<'all' | 'installed' | 'update_available'>('all');
  const [pendingUninstallApp, setPendingUninstallApp] = useState<AppInfo | null>(null);

  // SSE Reference to close it on unmount
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    loadApps();
    
    // Setup SSE connection
    const es = getSSEEventSource();
    eventSourceRef.current = es;

    es.onopen = () => {
      console.log('SSE Connected');
    };

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleServerEvent(data);
      } catch (e) {
        // Ignore parse errors (e.g. heartbeat)
      }
    };

    es.onerror = (err) => {
      console.error('SSE Error', err);
      // Optional: Reconnect logic is usually handled by browser, but we might want to log it
    };

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, []);

  useEffect(() => {
    if (!isUpdatingStore) return;

    let retries = 0;
    const maxRetries = 30;
    
    setProgressState({
        step: 'updating_store',
        progress: 100,
        message: '商店正在更新，请稍候...'
    });
    setProgressVisible(true);

    const poll = async () => {
      try {
        await fetchStatus();
        window.location.reload();
      } catch (error) {
        retries++;
        if (retries > maxRetries) {
          setProgressState({
            step: 'error',
            progress: 100,
            message: '重启超时，请手动刷新页面',
            type: 'error'
          });
          return;
        }
        
        setProgressState({
            step: 'restarting',
            progress: 100,
            message: '正在重启...'
        });
        
        setTimeout(poll, 2000);
      }
    };

    setTimeout(poll, 2000);

  }, [isUpdatingStore]);

  const handleServerEvent = (data: any) => {
    if (data.type === 'connected') return;

    if (data.type === 'self_update') {
        setIsUpdatingStore(true);
        if (eventSourceRef.current) {
            eventSourceRef.current.close();
            eventSourceRef.current = null;
        }
        return;
    }

    // If we have an active app, and this event is for that app (or missing app field implies current context)
    // We prioritize events that match the active app.
    if (activeApp && (data.app === activeApp || !data.app)) {
        if (data.type === 'error') {
            toast.error(data.error || '发生未知错误');
            setProgressVisible(false);
            setActiveApp(null);
            loadApps(); // Refresh to ensure consistent state
            return;
        }
        
        if (data.step === 'done') {
            setProgressVisible(false);
            setActiveApp(null);
            loadApps(); // Refresh list
            return;
        }

        setProgressState({
            step: data.step || 'processing',
            progress: data.progress || 0,
            message: data.message || translateStep(data.step),
            type: data.type
        });
    } else if (data.app && !activeApp) {
        // Background update or from another session?
        // We could show a toast, but for now ignore.
    }
  };

  const translateStep = (step?: string) => {
      switch(step) {
          case 'downloading': return '正在下载...';
          case 'installing': return '正在安装...';
          case 'verifying': return '正在验证...';
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

  const startOperation = (app: AppInfo, action: string) => {
      setActiveApp(app.appname);
      setProgressVisible(true);
      setProgressState({
          step: 'starting',
          progress: 0,
          message: `${action} ${app.display_name}...`
      });
  };

  const handleInstall = async (app: AppInfo) => {
    startOperation(app, '正在安装');
    try {
        await installApp(app.appname);
        // Wait for SSE to finish
    } catch (error) {
        console.error(error);
        toast.error('安装请求失败');
        setProgressVisible(false);
        setActiveApp(null);
    }
  };

  const handleUpdate = async (app: AppInfo) => {
    startOperation(app, '正在更新');
    try {
        await updateApp(app.appname);
    } catch (error) {
        console.error(error);
        toast.error('更新请求失败');
        setProgressVisible(false);
        setActiveApp(null);
    }
  };

  const handleUninstall = (app: AppInfo) => {
    setPendingUninstallApp(app);
  };

  const confirmUninstall = async () => {
    if (!pendingUninstallApp) return;
    const app = pendingUninstallApp;
    setPendingUninstallApp(null);
    
    startOperation(app, '正在卸载');
    try {
        await uninstallApp(app.appname);
    } catch (error) {
        console.error(error);
        toast.error('卸载请求失败');
        setProgressVisible(false);
        setActiveApp(null);
    }
  };

  const filteredApps = apps.filter(app => {
    if (activeFilter === 'all') return true;
    if (activeFilter === 'installed') return app.installed;
    if (activeFilter === 'update_available') return app.has_update;
    return true;
  });

  const counts = {
      all: apps.length,
      installed: apps.filter(a => a.installed).length,
      update_available: apps.filter(a => a.has_update).length
  };

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col md:flex-row">
      <aside className="hidden md:flex flex-col w-64 bg-card border-r border-border h-screen sticky top-0">
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
       </aside>

      <div className="flex-1 flex flex-col min-h-screen">
        <div className="md:hidden bg-card border-b border-border p-4 sticky top-0 z-20 flex items-center justify-between">
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

        <header className="hidden md:flex bg-card border-b border-border px-8 py-4 justify-between items-center sticky top-0 z-10">
           <h2 className="text-lg font-medium">
              {activeFilter === 'all' && '全部应用'}
              {activeFilter === 'installed' && '已安装应用'}
              {activeFilter === 'update_available' && '可用更新'}
           </h2>
           <div className="flex items-center space-x-4">
               <Button 
                 onClick={handleCheck} 
                 disabled={checking}
               >
                 {checking ? (
                   <>
                     <RefreshCw className={`mr-2 h-4 w-4 ${checking ? 'animate-spin' : ''}`} />
                     {checking ? '检查中...' : '立即检查'}
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
             filterType={activeFilter}
           />
        </main>
      </div>

      <ProgressOverlay
        visible={progressVisible}
        message={progressState.message || ''}
        progress={progressState.progress || 0}
      />
      
      {settingsVisible && (
        <SettingsDialog
            visible={settingsVisible}
            onClose={() => setSettingsVisible(false)}
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

      <Toaster />
    </div>
  );
};

export default App;
