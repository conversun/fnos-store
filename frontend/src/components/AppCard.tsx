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
  const serviceUrl = app.service_port ? `http://${window.location.hostname}:${app.service_port}` : null;

  // Status mapping
  const getStatusBadge = () => {
    if (!isInstalled) {
      return <span className="px-2 py-1 text-xs font-semibold bg-gray-100 text-gray-600 rounded">未安装</span>;
    }
    if (canUpdate) {
      return <span className="px-2 py-1 text-xs font-semibold bg-orange-100 text-orange-600 rounded">有更新</span>;
    }
    return <span className="px-2 py-1 text-xs font-semibold bg-green-100 text-green-600 rounded">已安装</span>;
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'running': return '运行中';
      case 'stopped': return '已停止';
      case 'installing': return '安装中';
      case 'uninstalling': return '卸载中';
      case 'updating': return '更新中';
      default: return status || '未知';
    }
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 flex flex-col h-full">
      <div className="flex items-start space-x-4 mb-4">
        {app.icon_url ? (
          <img src={app.icon_url} alt={app.display_name} className="w-16 h-16 rounded-md object-contain bg-gray-50" />
        ) : (
          <div className="w-16 h-16 bg-gray-200 dark:bg-gray-700 rounded-md flex items-center justify-center text-2xl font-bold text-gray-500">
            {app.display_name.charAt(0)}
          </div>
        )}
        
        <div className="flex-1 min-w-0">
          <div className="flex justify-between items-start">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate" title={app.display_name}>
              {app.display_name}
            </h3>
            {getStatusBadge()}
          </div>
          
          <div className="text-sm text-gray-500 dark:text-gray-400 mt-1 space-y-1">
            <div className="flex items-center justify-between">
              <span>版本:</span>
              <span className="font-mono">{isInstalled ? app.installed_version : app.latest_version}</span>
            </div>
            {canUpdate && (
              <div className="flex items-center justify-between text-orange-600">
                <span>最新:</span>
                <span className="font-mono">{app.latest_version}</span>
              </div>
            )}
             {isInstalled && (
              <div className="flex items-center justify-between">
                <span>状态:</span>
                <span className={app.status === 'running' ? 'text-green-500' : 'text-gray-500'}>
                  {getStatusText(app.status)}
                </span>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="mt-auto space-y-3">
        {/* Action Buttons */}
        <div className="grid grid-cols-2 gap-2">
            {!isInstalled ? (
                <button
                onClick={() => onInstall(app)}
                className="col-span-2 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition font-medium"
                >
                安装
                </button>
            ) : canUpdate ? (
                <>
                <button
                    onClick={() => onUpdate(app)}
                    className="px-4 py-2 bg-orange-500 text-white rounded hover:bg-orange-600 transition font-medium"
                >
                    更新
                </button>
                <button
                    onClick={() => onUninstall(app)}
                    className="px-4 py-2 text-red-500 hover:bg-red-50 rounded transition border border-gray-200"
                >
                    卸载
                </button>
                </>
            ) : (
                <>
                {serviceUrl && app.status === 'running' ? (
                   <a
                   href={serviceUrl}
                   target="_blank"
                   rel="noopener noreferrer"
                   className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition font-medium text-center"
                 >
                   打开
                 </a>
                ) : (
                    <button disabled className="px-4 py-2 bg-gray-100 text-gray-400 rounded cursor-not-allowed">
                        已是最新
                    </button>
                )}
                 <button
                    onClick={() => onUninstall(app)}
                    className="px-4 py-2 text-red-500 hover:bg-red-50 rounded transition border border-gray-200"
                >
                    卸载
                </button>
                </>
            )}
        </div>
        
        {/* Links */}
         {app.homepage && (
            <div className="text-center pt-2">
                <a 
                    href={app.homepage} 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="text-xs text-gray-400 hover:text-blue-500 transition"
                >
                    访问官网
                </a>
            </div>
        )}
      </div>
    </div>
  );
};

export default AppCard;
