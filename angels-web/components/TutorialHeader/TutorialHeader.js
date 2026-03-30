import {
  Header,
  HeaderContainer,
  HeaderName,
  HeaderMenuButton,
  HeaderGlobalBar,
  HeaderGlobalAction,
  SkipToContent,
  SideNav,
  SideNavItems
} from '@carbon/react';
import { Switcher, Notification, UserAvatar } from '@carbon/icons-react';
import { useState } from 'react';
import Tree from '@/components/tree/Tree';
import { usePathname, useRouter } from 'next/navigation';

const TutorialHeader = () => {
  const [isSideNavExpanded, setIsSideNavExpanded] = useState(false); // Start collapsed
  const [expandedNodes, setExpandedNodes] = useState({});
  const pathname = usePathname();
  const router = useRouter();

  const handleDataFromChild = () => {
    setIsSideNavExpanded(false);
  };

  const handleToggle = (label, isExpanded) => {
    setExpandedNodes((prev) => ({
      ...prev,
      [label]: isExpanded,
    }));
  };

  const handleNavigation = (path) => {
    setIsSideNavExpanded(false); // Close SideNav on route change
    router.push(path);
  };

  return (
    <HeaderContainer
      render={() => (
        <Header
          aria-label="Carbon Tutorial"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between', // Ensures content is spaced out
          }}
        >
          <SkipToContent />

          {/* Container for HeaderMenuButton and HeaderName */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <HeaderMenuButton
              aria-label="Open menu"
              onClick={() => setIsSideNavExpanded(!isSideNavExpanded)} // Toggle SideNav on click
              isActive={isSideNavExpanded}
              style={{
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                marginRight: '16px',
              }}
            />
            <HeaderName href="/" prefix="">
              Путь Ангелов
            </HeaderName>
          </div>

          <SideNav expanded={isSideNavExpanded} isPersistent={false}>
            <SideNavItems>
              <Tree expandedNodes={expandedNodes} onToggle={handleToggle} onHandleSidenav={handleDataFromChild} />
            </SideNavItems>
          </SideNav>

          <HeaderGlobalBar>
            <HeaderGlobalAction
              aria-label="Notifications"
              tooltipAlignment="center"
              className="action-icons"
              onClick={() => handleNavigation('/note')}
            >
              <Notification size={20} />
            </HeaderGlobalAction>
            <HeaderGlobalAction
              aria-label="User Avatar"
              tooltipAlignment="center"
              className="action-icons"
              onClick={() => handleNavigation('/filterednote')}
            >
              <UserAvatar size={20} />
            </HeaderGlobalAction>
            <HeaderGlobalAction
              aria-label="App Switcher"
              tooltipAlignment="end"
              onClick={() => handleNavigation('/468')}
            >
              <Switcher size={20} />
            </HeaderGlobalAction>
          </HeaderGlobalBar>
        </Header>
      )}
    />
  );
};

export default TutorialHeader;
