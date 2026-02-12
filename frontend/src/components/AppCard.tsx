import React from 'react';
import type { AppInfo } from '../api/client';

interface AppCardProps {
  app: AppInfo;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  onUninstall: (app: AppInfo) => void;
}

const AppCard: React.FC<AppCardProps> = ({ app, onInstall, onUpdate, onUninstall }) => {
  const isInstalled = app.installed;
  const canUpdate = isInstalled && app.has_update;

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 flex items-center space-x-4">
      <div className="w-16 h-16 bg-gray-200 dark:bg-gray-700 rounded-md flex items-center justify-center text-2xl font-bold text-gray-500">
        {app.display_name.charAt(0)}
      </div>
      
      <div className="flex-1">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{app.display_name}</h3>
        <div className="text-sm text-gray-500 dark:text-gray-400">
          {isInstalled ? (
            <>
              当前版本: {app.installed_version}
              {canUpdate && <span className="ml-2 text-red-500">→ {app.latest_version}</span>}
            </>
          ) : (
            <>最新版本: {app.latest_version}</>
          )}
        </div>
        {isInstalled && (
          <div className="text-xs text-gray-400 mt-1">
            状态: <span className={app.status === 'running' ? 'text-green-500' : 'text-gray-500'}>{app.status || '未知'}</span>
          </div>
        )}
      </div>

      <div className="flex space-x-2">
        {canUpdate && (
          <button
            onClick={() => onUpdate(app)}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition"
          >
            更新
          </button>
        )}
        {!isInstalled && (
          <button
            onClick={() => onInstall(app)}
            className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition"
          >
            安装
          </button>
        )}
        {isInstalled && !canUpdate && (
          <button
            disabled
            className="px-4 py-2 bg-gray-100 text-gray-400 rounded cursor-not-allowed"
          >
            已最新
          </button>
        )}
        {isInstalled && (
            <button
            onClick={() => onUninstall(app)}
            className="px-3 py-2 text-red-500 hover:bg-red-50 rounded transition"
            title="卸载"
            >
            卸载
            </button>
        )}
      </div>
    </div>
  );
};

export default AppCard;
