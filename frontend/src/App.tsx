import React, { useState, useEffect, useRef } from 'react';
import AppList from './components/AppList';
import ProgressOverlay from './components/ProgressOverlay';
import SettingsDialog from './components/SettingsDialog';
import { fetchApps, triggerCheck, installApp, updateApp, uninstallApp, getSSEEventSource } from './api/client';
import type { AppInfo, UpdateProgress } from './api/client';

const App: React.FC = () => {
  const [apps, setApps] = useState<AppInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [checking, setChecking] = useState<boolean>(false);
  const [lastCheck, setLastCheck] = useState<string>('');
  
  // Progress state
  const [progressVisible, setProgressVisible] = useState(false);
  const [progressState, setProgressState] = useState<UpdateProgress>({ step: '', progress: 0, message: '' });
  const [activeApp, setActiveApp] = useState<string | null>(null);

  const [settingsVisible, setSettingsVisible] = useState(false);
  const [checkInterval, setCheckInterval] = useState(24);

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

  const handleServerEvent = (data: any) => {
    // Check if this event is relevant to us
    if (data.type === 'connected') return;

    // If we have an active app, and this event is for that app (or missing app field implies current context)
    // We prioritize events that match the active app.
    if (activeApp && (data.app === activeApp || !data.app)) {
        if (data.type === 'error') {
            alert(`Error: ${data.error || 'Unknown error'}`);
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
      alert('检查更新失败');
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
        alert('安装请求失败');
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
        alert('更新请求失败');
        setProgressVisible(false);
        setActiveApp(null);
    }
  };

  const handleUninstall = async (app: AppInfo) => {
    if (!confirm(`确定要卸载 ${app.display_name} 吗？`)) return;
    
    startOperation(app, '正在卸载');
    try {
        await uninstallApp(app.appname);
    } catch (error) {
        console.error(error);
        alert('卸载请求失败');
        setProgressVisible(false);
        setActiveApp(null);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-white flex flex-col">
      <header className="bg-white dark:bg-gray-800 shadow-sm sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
          <div>
            <h1 className="text-2xl font-bold text-blue-600 dark:text-blue-400">fnOS Apps Store</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              上次检查: {lastCheck ? new Date(lastCheck).toLocaleString() : '从未'}
            </p>
          </div>
          <div className="flex space-x-3 items-center">
             <button
              onClick={() => setSettingsVisible(true)}
              className="p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-full transition"
              title="设置"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>
            <button
              onClick={handleCheck}
              disabled={checking}
              className={`px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition flex items-center ${
                checking ? 'opacity-70 cursor-wait' : ''
              }`}
            >
              {checking && (
                <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              )}
              {checking ? '检查中...' : '立即检查'}
            </button>
          </div>
        </div>
      </header>

      <main className="flex-grow max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
        <AppList
          apps={apps}
          loading={loading}
          onInstall={handleInstall}
          onUpdate={handleUpdate}
          onUninstall={handleUninstall}
        />
      </main>

      <footer className="bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700 mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
          fnOS Apps Store &copy; 2026 - Designed for fnOS
        </div>
      </footer>

      <ProgressOverlay
        visible={progressVisible}
        message={progressState.message || ''}
        progress={progressState.progress || 0}
      />
      
      {settingsVisible && (
        <SettingsDialog
            visible={settingsVisible}
            onClose={() => setSettingsVisible(false)}
            checkInterval={checkInterval}
            onCheckIntervalChange={setCheckInterval}
        />
      )}
    </div>
  );
};

export default App;
