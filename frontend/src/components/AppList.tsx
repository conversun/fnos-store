import React from 'react';
import type { AppInfo, AppOperation } from '../api/client';
import AppCard from './AppCard';
import { PackageSearch, CheckCircle2, RefreshCw, Search } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';

interface AppListProps {
  apps: AppInfo[];
  loading: boolean;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  onUninstall: (app: AppInfo) => void;
  onDetail: (app: AppInfo) => void;
  onCancelOp?: (app: AppInfo) => void;
  filterType?: string;
  appOperations?: Map<string, AppOperation>;
  searchQuery?: string;
}

const getEmptyMessage = (filterType?: string) => {
  switch (filterType) {
    case 'installed':
      return { icon: CheckCircle2, text: '暂无已安装的应用' };
    case 'update_available':
      return { icon: RefreshCw, text: '所有应用都是最新版本' };
    default:
      return { icon: PackageSearch, text: '暂无可用应用' };
  }
};

const AppList: React.FC<AppListProps> = ({ apps, loading, onInstall, onUpdate, onUninstall, onDetail, onCancelOp, filterType, appOperations, searchQuery }) => {
  if (loading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="flex flex-col space-y-3">
            <Skeleton className="h-[125px] w-full rounded-xl" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-[250px]" />
              <Skeleton className="h-4 w-[200px]" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (apps.length === 0) {
    if (searchQuery?.trim()) {
      return (
        <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
          <Search className="h-12 w-12 mb-4 opacity-40" />
          <p className="text-sm">未找到匹配「{searchQuery.trim()}」的应用</p>
        </div>
      );
    }
    const empty = getEmptyMessage(filterType);
    const Icon = empty.icon;
    return (
      <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
        <Icon className="h-12 w-12 mb-4 opacity-40" />
        <p className="text-sm">{empty.text}</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {apps.map((app) => (
        <AppCard
          key={app.appname}
          app={app}
          operation={appOperations?.get(app.appname)}
          onInstall={onInstall}
          onUpdate={onUpdate}
          onUninstall={onUninstall}
          onDetail={onDetail}
          onCancelOp={onCancelOp}
        />
      ))}
    </div>
  );
};

export default AppList;
