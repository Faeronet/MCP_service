'use client';

import TutorialHeader from '@/components/TutorialHeader/TutorialHeader';
import { Content, Theme } from '@carbon/react';
// import './custom-theme.css';

export function Providers({ children }) {
  return (
    <div>
      <Theme theme="g100">
          <TutorialHeader />
      </Theme>
      <div style={{ marginTop: '50px' }}>{children}</div>
    </div>
  );
}
