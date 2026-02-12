import React, { useState, useEffect } from 'react';
import AppList from './components/AppList';
import ProgressOverlay from './components/ProgressOverlay';
import SettingsDialog from './components/SettingsDialog';
import { fetchApps, triggerCheck, installApp, updateApp, uninstallApp } from './api/client';
import type { AppInfo } from './api/client';

const App: React.FC = () => {
  const [apps, setApps] = useState<AppInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [checking, setChecking] = useState<boolean>(false);
  const [lastCheck, setLastCheck] = useState<string>('');
  const [updateProgress, setUpdateProgress] = useState<{ visible: boolean; message: string; progress: number }>({
    visible: false,
    message: '',
    progress: 0,
  });
  const [settingsVisible, setSettingsVisible] = useState(false);
  const [checkInterval, setCheckInterval] = useState(24);

  useEffect(() => {
    loadApps();
  }, []);

  const loadApps = async () => {
    setLoading(true);
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
    } finally {
      setChecking(false);
    }
  };

  const handleInstall = async (app: AppInfo) => {
    setUpdateProgress({ visible: true, message: `正在安装 ${app.display_name}...`, progress: 0 });
    for (let i = 0; i <= 100; i += 10) {
      setUpdateProgress((prev) => ({ ...prev, progress: i }));
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    await installApp(app.appname);
    setUpdateProgress({ visible: false, message: '', progress: 0 });
    loadApps();
  };

  const handleUpdate = async (app: AppInfo) => {
    setUpdateProgress({ visible: true, message: `正在更新 ${app.display_name}...`, progress: 0 });
    for (let i = 0; i <= 100; i += 10) {
      setUpdateProgress((prev) => ({ ...prev, progress: i }));
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    await updateApp(app.appname);
    setUpdateProgress({ visible: false, message: '', progress: 0 });
    loadApps();
  };

  const handleUninstall = async (app: AppInfo) => {
    if (!confirm(`确定要卸载 ${app.display_name} 吗？`)) return;
    setUpdateProgress({ visible: true, message: `正在卸载 ${app.display_name}...`, progress: 0 });
    for (let i = 0; i <= 100; i += 20) {
      setUpdateProgress((prev) => ({ ...prev, progress: i }));
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    await uninstallApp(app.appname);
    setUpdateProgress({ visible: false, message: '', progress: 0 });
    loadApps();
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
        visible={updateProgress.visible}
        message={updateProgress.message}
        progress={updateProgress.progress}
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
