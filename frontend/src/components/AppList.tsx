import React from 'react';
import type { AppInfo } from '../api/client';
import AppCard from './AppCard';

interface AppListProps {
  apps: AppInfo[];
  loading: boolean;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  onUninstall: (app: AppInfo) => void;
}

const AppList: React.FC<AppListProps> = ({ apps, loading, onInstall, onUpdate, onUninstall }) => {
  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900 dark:border-white"></div>
      </div>
    );
  }

  if (apps.length === 0) {
    return (
      <div className="text-center text-gray-500 py-10">
        暂无应用
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 p-4">
      {apps.map((app) => (
        <AppCard
          key={app.appname}
          app={app}
          onInstall={onInstall}
          onUpdate={onUpdate}
          onUninstall={onUninstall}
        />
      ))}
    </div>
  );
};

export default AppList;
