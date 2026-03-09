import React from 'react';
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ExternalLink, Package } from 'lucide-react';
import type { RecommendedApp } from '../api/client';
import { cn } from "@/lib/utils";

interface RecommendedAppCardProps {
  app: RecommendedApp;
}

const RecommendedAppCard: React.FC<RecommendedAppCardProps> = ({ app }) => {
  return (
    <Card className={cn(
      "relative overflow-hidden border border-border/30 bg-card shadow-[0_1px_3px_0_rgb(0_0_0/0.04)] rounded-xl"
    )}>
      <div className="p-4 flex flex-col h-full gap-3">
        <div className="flex items-start gap-3">
          <div className="shrink-0">
            <div className="w-11 h-11 bg-muted/60 rounded-xl flex items-center justify-center text-muted-foreground">
              <Package className="h-5 w-5 opacity-40" />
            </div>
          </div>

          <div className="flex-1 min-w-0 flex flex-col gap-0.5">
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-1.5 min-w-0">
                <h3 className="font-semibold text-sm leading-tight text-foreground truncate" title={app.display_name}>
                  {app.display_name}
                </h3>
              </div>
            </div>

            <div className="flex items-center flex-wrap gap-x-1.5 text-xs text-muted-foreground">
              {app.latest_version && <span>v{app.latest_version}</span>}
            </div>
          </div>
        </div>

        {app.description && (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
            {app.description}
          </p>
        )}

        <div className="flex-1" />

        <div className="flex items-center justify-end gap-2 pt-2 border-t border-border/20">
          <Button
            size="sm"
            onClick={() => window.open(app.source_url, '_blank')}
            variant="outline"
            className="rounded-full px-3.5 h-7 text-xs font-medium"
          >
            <ExternalLink className="mr-1 h-3.5 w-3.5" />
            查看来源
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default RecommendedAppCard;
