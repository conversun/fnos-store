import React from 'react';

interface ProgressOverlayProps {
  visible: boolean;
  message: string;
  progress: number;
  onCancel?: () => void;
}

const ProgressOverlay: React.FC<ProgressOverlayProps> = ({ visible, message, progress, onCancel }) => {
  if (!visible) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-sm w-full shadow-xl">
        <h3 className="text-lg font-bold mb-4 text-gray-900 dark:text-white">正在处理...</h3>
        <div className="mb-4">
          <div className="flex justify-between text-sm text-gray-600 dark:text-gray-400 mb-1">
            <span>{message}</span>
            <span>{Math.round(progress)}%</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2.5 dark:bg-gray-700">
            <div
              className="bg-blue-600 h-2.5 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            ></div>
          </div>
        </div>
        {onCancel && (
          <div className="text-right">
            <button
              onClick={onCancel}
              className="text-gray-500 hover:text-gray-700 text-sm underline"
            >
              取消
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

export default ProgressOverlay;
