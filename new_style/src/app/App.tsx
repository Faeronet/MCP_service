import { useState } from 'react';
import { motion } from 'motion/react';
import { FileText, Briefcase, ScrollText, BarChart3, LogOut, Search, Filter, Database, Activity } from 'lucide-react';

// Mock log data
const mockLogs = [
  {
    time: '2026-02-25T11:54:19.831Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'a4f29b12',
    message: '2026-02-25 11:54:19.831 UTC [28] LOG: checkpoint complete: wrote 323 buffers (2.0%); 0 WAL file(s) added, 0 removed, 0 recycled; write=32.362 s, sync=0.015 s, total=32.395 s; sync files=14, longest=0.006 s, average=0.001 s; distance=21d4 kB, estimate=4542 kB, lsn=0/14693D08, redo lsn=0/14617928'
  },
  {
    time: '2026-02-25T11:54:03.100Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'b8e31c45',
    message: 'level=info ts=2026-02-25T11:54:03.015110103Z caller=metrics.go:159 component=frontend org_id=fake traceID=11119b00c62528a2c msg="executing query" type="range length=2m0.000002545s query_hash=43d4a0e3'
  },
  {
    time: '2026-02-25T11:54:03.100Z',
    level: 'warning',
    service: 'containerlogs',
    requestId: 'c2f45d78',
    message: 'level=info ts=2026-02-25T11:54:03.014236576Z caller=querier org_id=fake traceID=11119b00c62528a2c msg="executing query" type="range query="[job=\"-\", \"*\"]" length=2m0.000002545s steps=15 query_hash=453119268'
  },
  {
    time: '2026-02-25T11:54:03.100Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'd5a67e91',
    message: 'level=info ts=2026-02-25T11:54:03.012841126Z caller=engine.go:234 component=querier org_id=fake traceID=11119b00c62528a2c msg="executing query" type="range query="[job=\"-\", \"*\"]" length=2m0.000002545s steps=15 query_hash=453119268'
  },
  {
    time: '2026-02-25T11:53:47.437Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'e9b82fa3',
    message: '2026-02-25 11:53:47.437 UTC [28] LOG: checkpoint starting: time'
  },
  {
    time: '2026-02-25T11:53:47.437Z',
    level: 'error',
    service: 'containerlogs',
    requestId: 'f1c93gb5',
    message: '2026-02-25 11:53:47.437 UTC [28] LOG: checkpoint starting: time'
  },
  {
    time: '2026-02-25T11:53:37.420Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'a2d04hc7',
    message: 'level=info ts=2026-02-25T11:53:37.369520819Z caller=compactor.go:688 msg="finished compacting table" table-name=index_20508'
  },
  {
    time: '2026-02-25T11:53:37.400Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'b3e15id9',
    message: 'level=info ts=2026-02-25T11:53:37.369520819Z caller=compactor.go:688 msg="finished compacting table" table-name=index_20508'
  },
  {
    time: '2026-02-25T11:53:37.400Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'c4f26je1',
    message: 'level=info ts=2026-02-25T11:53:37.369497018Z caller=table.go:132 table-name=index_20508 msg="listed files" count=1'
  },
  {
    time: '2026-02-25T11:53:37.400Z',
    level: 'info',
    service: 'containerlogs',
    requestId: 'd5g37kf3',
    message: 'level=info ts=2026-02-25T11:53:37.369497018Z caller=table.go:132 table-name=index_20508 msg="listed files" count=1'
  }
];

const navItems = [
  { name: 'Docs', icon: FileText, active: false },
  { name: 'Jobs', icon: Briefcase, active: false },
  { name: 'Logs', icon: ScrollText, active: true },
  { name: 'Grafana', icon: BarChart3, active: false }
];

export default function App() {
  const [serviceFilter, setServiceFilter] = useState('');
  const [requestIdFilter, setRequestIdFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('');
  const [hoveredRow, setHoveredRow] = useState<number | null>(null);

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'error':
        return 'text-red-400';
      case 'warning':
        return 'text-amber-400';
      default:
        return 'text-cyan-400';
    }
  };

  const getLevelBg = (level: string) => {
    switch (level) {
      case 'error':
        return 'bg-red-500/10 border-red-500/30';
      case 'warning':
        return 'bg-amber-500/10 border-amber-500/30';
      default:
        return 'bg-cyan-500/10 border-cyan-500/30';
    }
  };

  return (
    <div className="size-full bg-gradient-to-br from-[#020818] via-[#030a1f] to-black text-gray-100 overflow-hidden relative">
      {/* Animated background grid */}
      <div className="absolute inset-0 opacity-10">
        <div className="absolute inset-0" style={{
          backgroundImage: 'linear-gradient(rgba(6, 182, 212, 0.05) 1px, transparent 1px), linear-gradient(90deg, rgba(6, 182, 212, 0.05) 1px, transparent 1px)',
          backgroundSize: '50px 50px'
        }}></div>
      </div>

      {/* Glowing orbs */}
      <motion.div
        className="absolute top-1/4 left-1/4 w-96 h-96 bg-cyan-500/15 rounded-full blur-[120px]"
        animate={{
          scale: [1, 1.2, 1],
          opacity: [0.2, 0.4, 0.2],
        }}
        transition={{
          duration: 8,
          repeat: Infinity,
          ease: "easeInOut"
        }}
      />
      <motion.div
        className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-purple-500/15 rounded-full blur-[120px]"
        animate={{
          scale: [1.2, 1, 1.2],
          opacity: [0.4, 0.2, 0.4],
        }}
        transition={{
          duration: 10,
          repeat: Infinity,
          ease: "easeInOut"
        }}
      />

      <div className="size-full flex relative z-10">
        {/* Sidebar */}
        <motion.div
          initial={{ x: -100, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          transition={{ duration: 0.5 }}
          className="w-48 bg-[#071A71]/30 backdrop-blur-xl border-r border-cyan-500/20 flex flex-col relative"
        >
          {/* Glowing edge */}
          <div className="absolute top-0 right-0 w-px h-full bg-gradient-to-b from-transparent via-cyan-500/50 to-transparent"></div>

          {/* Admin header */}
          <div className="p-6 border-b border-cyan-500/20">
            <motion.div
              className="flex items-center gap-2"
              whileHover={{ scale: 1.02 }}
            >
              <Database className="w-5 h-5 text-cyan-400" />
              <h1 className="font-bold text-lg bg-gradient-to-r from-cyan-400 to-purple-400 bg-clip-text text-transparent">
                Admin
              </h1>
            </motion.div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 py-4 px-3">
            {navItems.map((item, index) => (
              <motion.button
                key={item.name}
                initial={{ x: -50, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ delay: index * 0.1 }}
                className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg mb-2 transition-all duration-300 relative group ${
                  item.active
                    ? 'bg-gradient-to-r from-cyan-500/20 to-purple-500/20 text-cyan-400 shadow-lg shadow-cyan-500/20'
                    : 'text-gray-400 hover:text-cyan-300 hover:bg-white/5'
                }`}
                whileHover={{ x: 4 }}
                whileTap={{ scale: 0.98 }}
              >
                {item.active && (
                  <motion.div
                    layoutId="activeTab"
                    className="absolute inset-0 border border-cyan-500/30 rounded-lg"
                  />
                )}
                <item.icon className="w-4 h-4 relative z-10" />
                <span className="relative z-10">{item.name}</span>
              </motion.button>
            ))}
          </nav>

          {/* Logout button */}
          <div className="p-3 border-t border-cyan-500/20">
            <motion.button
              className="w-full bg-gradient-to-r from-red-500/10 to-orange-500/10 border border-red-500/30 text-red-400 px-4 py-2 rounded-lg flex items-center justify-center gap-2 hover:from-red-500/20 hover:to-orange-500/20 transition-all duration-300"
              whileHover={{ scale: 1.02, boxShadow: '0 0 20px rgba(239, 68, 68, 0.3)' }}
              whileTap={{ scale: 0.98 }}
            >
              <LogOut className="w-4 h-4" />
              Logout
            </motion.button>
          </div>
        </motion.div>

        {/* Main content */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Header */}
          <motion.div
            initial={{ y: -50, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            transition={{ duration: 0.5 }}
            className="p-8 border-b border-cyan-500/20 bg-black/40 backdrop-blur-sm relative"
          >
            <div className="absolute inset-0 bg-gradient-to-r from-cyan-500/5 to-purple-500/5"></div>
            <div className="relative z-10">
              <div className="flex items-center gap-3 mb-6">
                <Activity className="w-8 h-8 text-cyan-400" />
                <h2 className="text-3xl font-bold bg-gradient-to-r from-cyan-400 via-blue-400 to-purple-400 bg-clip-text text-transparent">
                  Logs
                </h2>
              </div>

              {/* Search filters */}
              <div className="flex gap-4">
                <motion.div
                  className="flex-1 relative group"
                  whileHover={{ scale: 1.01 }}
                >
                  <input
                    type="text"
                    placeholder="Service"
                    value={serviceFilter}
                    onChange={(e) => setServiceFilter(e.target.value)}
                    className="w-full bg-black/60 backdrop-blur-sm border border-cyan-500/30 rounded-lg px-4 py-2.5 text-gray-300 placeholder-gray-500 focus:outline-none focus:border-cyan-400 focus:shadow-lg focus:shadow-cyan-500/20 transition-all duration-300"
                  />
                  <div className="absolute inset-0 rounded-lg bg-gradient-to-r from-cyan-500/0 via-cyan-500/10 to-cyan-500/0 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>
                </motion.div>

                <motion.div
                  className="flex-1 relative group"
                  whileHover={{ scale: 1.01 }}
                >
                  <input
                    type="text"
                    placeholder="Request ID"
                    value={requestIdFilter}
                    onChange={(e) => setRequestIdFilter(e.target.value)}
                    className="w-full bg-black/60 backdrop-blur-sm border border-cyan-500/30 rounded-lg px-4 py-2.5 text-gray-300 placeholder-gray-500 focus:outline-none focus:border-cyan-400 focus:shadow-lg focus:shadow-cyan-500/20 transition-all duration-300"
                  />
                  <div className="absolute inset-0 rounded-lg bg-gradient-to-r from-cyan-500/0 via-cyan-500/10 to-cyan-500/0 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>
                </motion.div>

                <motion.div
                  className="flex-1 relative group"
                  whileHover={{ scale: 1.01 }}
                >
                  <input
                    type="text"
                    placeholder="Level"
                    value={levelFilter}
                    onChange={(e) => setLevelFilter(e.target.value)}
                    className="w-full bg-black/60 backdrop-blur-sm border border-cyan-500/30 rounded-lg px-4 py-2.5 text-gray-300 placeholder-gray-500 focus:outline-none focus:border-cyan-400 focus:shadow-lg focus:shadow-cyan-500/20 transition-all duration-300"
                  />
                  <div className="absolute inset-0 rounded-lg bg-gradient-to-r from-cyan-500/0 via-cyan-500/10 to-cyan-500/0 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none"></div>
                </motion.div>

                <motion.button
                  className="bg-gradient-to-r from-cyan-500 to-blue-500 text-white px-6 py-2.5 rounded-lg font-medium hover:from-cyan-400 hover:to-blue-400 transition-all duration-300 flex items-center gap-2 shadow-lg shadow-cyan-500/30"
                  whileHover={{ scale: 1.05, boxShadow: '0 0 30px rgba(6, 182, 212, 0.5)' }}
                  whileTap={{ scale: 0.95 }}
                >
                  <Search className="w-4 h-4" />
                  Search
                </motion.button>
              </div>

              <p className="text-sm text-gray-500 mt-4">
                Индекс из Postgres (таблица db.logs_index). Для скрытия запросов в Loki — Grafana.
              </p>
            </div>
          </motion.div>

          {/* Logs table */}
          <div className="flex-1 overflow-auto p-6">
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.2 }}
              className="bg-black/30 backdrop-blur-sm border border-cyan-500/20 rounded-xl overflow-hidden shadow-2xl"
            >
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-cyan-500/20 bg-gradient-to-r from-cyan-500/10 to-purple-500/10">
                      <th className="text-left py-4 px-6 text-cyan-400 font-semibold">Time</th>
                      <th className="text-left py-4 px-6 text-cyan-400 font-semibold">Level</th>
                      <th className="text-left py-4 px-6 text-cyan-400 font-semibold">Service</th>
                      <th className="text-left py-4 px-6 text-cyan-400 font-semibold">Request ID</th>
                      <th className="text-left py-4 px-6 text-cyan-400 font-semibold">Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    {mockLogs.map((log, index) => (
                      <motion.tr
                        key={index}
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ delay: index * 0.05 }}
                        onMouseEnter={() => setHoveredRow(index)}
                        onMouseLeave={() => setHoveredRow(null)}
                        className={`border-b border-cyan-500/10 transition-all duration-300 ${
                          hoveredRow === index ? 'bg-cyan-500/5' : ''
                        }`}
                      >
                        <td className="py-3 px-6 text-gray-400 font-mono text-sm">
                          {new Date(log.time).toLocaleString('en-US', {
                            year: 'numeric',
                            month: '2-digit',
                            day: '2-digit',
                            hour: '2-digit',
                            minute: '2-digit',
                            second: '2-digit'
                          })}
                        </td>
                        <td className="py-3 px-6">
                          <span className={`inline-flex items-center px-3 py-1 rounded-md border text-xs font-semibold uppercase ${getLevelBg(log.level)} ${getLevelColor(log.level)}`}>
                            {log.level}
                          </span>
                        </td>
                        <td className="py-3 px-6 text-purple-400 font-mono text-sm">{log.service}</td>
                        <td className="py-3 px-6 text-blue-400 font-mono text-sm">{log.requestId}</td>
                        <td className="py-3 px-6 text-gray-400 text-sm font-mono max-w-2xl truncate">
                          {log.message}
                        </td>
                      </motion.tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </motion.div>
          </div>
        </div>
      </div>
    </div>
  );
}