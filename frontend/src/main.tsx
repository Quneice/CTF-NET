import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';

import ErrorBoundary from '@/components/ErrorBoundary';
import { router } from '@/router';

import './index.css';

// 应用入口：以 RouterProvider 挂载整个路由树，外层包错误边界
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <RouterProvider router={router} />
    </ErrorBoundary>
  </React.StrictMode>,
);
