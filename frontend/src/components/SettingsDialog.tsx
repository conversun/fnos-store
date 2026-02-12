import React, { useState } from 'react';

interface SettingsDialogProps {
  visible: boolean;
  onClose: () => void;
  checkInterval: number;
  onCheckIntervalChange: (interval: number) => void;
}

const SettingsDialog: React.FC<SettingsDialogProps> = ({
  visible,
  onClose,
  checkInterval,
  onCheckIntervalChange,
}) => {
  const [interval, setInterval] = useState(checkInterval);

  if (!visible) return null;

  const handleSave = () => {
    onCheckIntervalChange(interval);
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-white">设置</h2>
        <div className="mb-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            自动检查更新间隔 (小时)
          </label>
          <input
            type="number"
            min="1"
            max="168"
            value={interval}
            onChange={(e) => setInterval(Number(e.target.value))}
            className="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 dark:text-white focus:ring-2 focus:ring-blue-500"
          />
        </div>
        <div className="flex justify-end space-x-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-600 hover:text-gray-800 dark:text-gray-300 dark:hover:text-white transition"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition"
          >
            保存
          </button>
        </div>
      </div>
    </div>
  );
};

export default SettingsDialog;
