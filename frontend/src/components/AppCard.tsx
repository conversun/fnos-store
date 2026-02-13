import React from 'react';
import type { AppInfo } from '../api/client';
import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Download, RefreshCw, Trash2, ExternalLink, Globe, Package } from 'lucide-react';

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
      return <Badge variant="secondary">未安装</Badge>;
    }
    if (canUpdate) {
      return <Badge variant="destructive">有更新</Badge>;
    }
    return <Badge className="bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 hover:bg-green-100 dark:hover:bg-green-900">已安装</Badge>;
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
    <Card className="flex flex-col h-full hover:shadow-md transition-shadow">
      <CardHeader className="flex-row gap-4 space-y-0 p-4 pb-2">
        {app.icon_url ? (
          <img 
            src={app.icon_url} 
            alt={app.display_name} 
            className="w-16 h-16 rounded-md object-contain bg-gray-50" 
          />
        ) : (
          <div className="w-16 h-16 bg-gray-200 dark:bg-gray-700 rounded-md flex items-center justify-center text-gray-500">
            <Package className="h-8 w-8" />
          </div>
        )}
        
        <div className="flex-1 min-w-0">
          <div className="flex justify-between items-start">
            <h3 className="text-lg font-semibold truncate pr-2" title={app.display_name}>
              {app.display_name}
            </h3>
            {getStatusBadge()}
          </div>
          
          <div className="text-sm text-muted-foreground mt-1 space-y-1">
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
      </CardHeader>

      <CardContent className="p-4 pt-2 flex-1">
      </CardContent>

      <CardFooter className="p-4 pt-0 flex-col gap-3">
        <div className="grid grid-cols-2 gap-2 w-full">
            {!isInstalled ? (
                <Button 
                    onClick={() => onInstall(app)}
                    className="col-span-2 w-full"
                >
                    <Download className="mr-2 h-4 w-4" />
                    安装
                </Button>
            ) : canUpdate ? (
                <>
                <Button
                    onClick={() => onUpdate(app)}
                    className="bg-orange-500 hover:bg-orange-600 text-white"
                >
                    <RefreshCw className="mr-2 h-4 w-4" />
                    更新
                </Button>
                <Button
                    variant="ghost"
                    onClick={() => onUninstall(app)}
                    className="text-red-500 hover:text-red-600 hover:bg-red-50"
                >
                    <Trash2 className="mr-2 h-4 w-4" />
                    卸载
                </Button>
                </>
            ) : (
                <>
                {serviceUrl && app.status === 'running' ? (
                   <Button asChild variant="outline" className="text-green-600 hover:text-green-700 border-green-200 hover:bg-green-50">
                     <a
                       href={serviceUrl}
                       target="_blank"
                       rel="noopener noreferrer"
                     >
                       <ExternalLink className="mr-2 h-4 w-4" />
                       打开
                     </a>
                   </Button>
                ) : (
                    <Button variant="ghost" disabled className="bg-gray-100 text-gray-400 cursor-not-allowed">
                        已是最新
                    </Button>
                )}
                 <Button
                    variant="ghost"
                    onClick={() => onUninstall(app)}
                    className="text-red-500 hover:text-red-600 hover:bg-red-50"
                >
                    <Trash2 className="mr-2 h-4 w-4" />
                    卸载
                </Button>
                </>
            )}
        </div>
        
         {app.homepage && (
            <div className="text-center w-full">
                <a 
                    href={app.homepage} 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="text-xs text-muted-foreground hover:text-primary transition inline-flex items-center"
                >
                    <Globe className="mr-1 h-3 w-3" />
                    访问官网
                </a>
            </div>
        )}
      </CardFooter>
    </Card>
  );
};

export default AppCard;
