import React, { useState, useEffect } from 'react';
import {
  DataTable,
  TableContainer,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
  Button,
  TextInput,
  ComposedModal,
  ModalHeader,
  ModalBody,
  ModalFooter,
  InlineNotification,
} from '@carbon/react';
import { TrashCan } from '@carbon/icons-react'; // Importing the delete icon
import { angelNameToRu, timeDataWithRussianNames } from './angelNamesMap';

const TG_USERNAME_STORAGE_KEY = 'schedulerTelegramUsername';

const TimeTable = () => {
  const [timeData, setTimeData] = useState([]);
  const [filterValue, setFilterValue] = useState(''); // State for the filter input
  const [sortOrder, setSortOrder] = useState('asc'); // State for sorting order (asc or desc)
  const [scheduleModalOpen, setScheduleModalOpen] = useState(false);
  const [telegramUsername, setTelegramUsername] = useState('');
  const [scheduleSubmitting, setScheduleSubmitting] = useState(false);
  const [scheduleBanner, setScheduleBanner] = useState(null);
  const [scheduleModalError, setScheduleModalError] = useState('');

  useEffect(() => {
    const data = loadTimeDataFromLocalStorage();
    const formattedData = formatDataForTable(data);
    setTimeData(formattedData);
  }, []);

  useEffect(() => {
    try {
      const u = localStorage.getItem(TG_USERNAME_STORAGE_KEY);
      if (u) setTelegramUsername(u);
    } catch {
      /* ignore */
    }
  }, []);

  const loadTimeDataFromLocalStorage = () => {
    try {
      const data = localStorage.getItem('timeData');
      return data ? JSON.parse(data) : {};
    } catch {
      return {};
    }
  };

  const formatDataForTable = (data) => {
    const formattedData = [];

    // Iterate through each hashed key in localStorage
    Object.entries(data).forEach(([hashedKey, row]) => {
      if (!row || typeof row !== 'object') return;
      const { pageName, keyName, value, validation, message, show } = row;
      if (show) {
        formattedData.push({
          id: hashedKey,
          pageName,
          timeKey: keyName,
          value,
          validation: angelNameToRu(validation),
          message,
          actions: 'delete'
        });
      }
    });

    return formattedData;
  };

  const deleteEntry = (id) => {
    const updatedData = { ...loadTimeDataFromLocalStorage() };

    if (updatedData[id]) {
      delete updatedData[id]; // Remove the entry
    }

    // Save updated data back to local storage
    localStorage.setItem('timeData', JSON.stringify(updatedData));

    // Update the state to reflect the changes in the UI
    setTimeData(formatDataForTable(updatedData));
  };

  const downloadJson = () => {
    const data = timeDataWithRussianNames(loadTimeDataFromLocalStorage());
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'timeData.json';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const buildSchedulePayloadFromRows = (rows) => {
    const items = [];
    for (const entry of rows) {
      const ru = angelNameToRu(entry.validation);
      const time = String(entry.value || '').trim();
      if (!ru || !time) continue;
      items.push({
        validation: ru,
        name: ru,
        time,
        message: String(entry.message || '').trim(),
      });
    }
    return items;
  };

  const openScheduleModal = () => {
    setScheduleBanner(null);
    setScheduleModalError('');
    try {
      const u = localStorage.getItem(TG_USERNAME_STORAGE_KEY);
      if (u) setTelegramUsername(u);
    } catch {
      /* ignore */
    }
    setScheduleModalOpen(true);
  };

  const handleScheduleSubmit = async () => {
    const rows = timeData.filter((entry) => String(entry.value ?? '').includes(filterValue));
    const items = buildSchedulePayloadFromRows(rows);
    const u = telegramUsername.trim().replace(/^@/, '');
    if (!u) {
      setScheduleModalError('Укажите Telegram username без @.');
      return;
    }
    if (items.length === 0) {
      setScheduleModalError('Нет строк для отправки: включите напоминания на страницах ангелов и сохраните время.');
      return;
    }
    setScheduleSubmitting(true);
    setScheduleModalError('');
    try {
      const body = JSON.stringify({ telegram_username: u, items });
      const res = await fetch('/api/schedule', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
      });
      let data;
      try {
        data = await res.json();
      } catch {
        data = null;
      }
      if (res.ok && data?.accepted) {
        try {
          localStorage.setItem(TG_USERNAME_STORAGE_KEY, u);
        } catch {
          /* ignore */
        }
        const cnt = typeof data.scheduled_count === 'number' ? data.scheduled_count : items.length;
        setScheduleModalOpen(false);
        setScheduleBanner({ kind: 'success', title: 'Запланировано', subtitle: `Записей: ${cnt}.` });
      } else {
        const err = (data?.errors && data.errors[0]) || `Ошибка ${res.status}`;
        setScheduleModalError(err);
      }
    } catch (e) {
      setScheduleModalError(e?.message || 'Запрос не выполнен');
    } finally {
      setScheduleSubmitting(false);
    }
  };

  const headers = [
    { key: 'pageName', header: 'Страница' },
    { key: 'timeKey', header: 'Время ангела' },
    { key: 'value', header: 'Время уведомления' },
    { key: 'validation', header: 'Имя Ангела' },
    { key: 'message', header: 'Ситуация' },
    { key: 'actions', header: 'Удалить' }, // New column for delete action
  ];

  // Sort the timeData based on the value (time)
  const sortData = (data, order) => {
    return data.sort((a, b) => {
      const [hoursA, minutesA] = a.value.split(':').map(Number);
      const [hoursB, minutesB] = b.value.split(':').map(Number);

      if (order === 'asc') {
        return hoursA !== hoursB ? hoursA - hoursB : minutesA - minutesB;
      } else {
        return hoursA !== hoursB ? hoursB - hoursA : minutesB - minutesA;
      }
    });
  };

  // Handle sorting when the header is clicked
  const handleSort = () => {
    const newSortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
    setSortOrder(newSortOrder);
    setTimeData(sortData([...timeData], newSortOrder)); // Sort and update the timeData
  };

  // Filter the data based on the filterValue
  const filteredData = timeData.filter((entry) =>
    String(entry.value ?? '').includes(filterValue)
  );

  return (
    <div>
      {scheduleBanner && (
        <div style={{ marginBottom: '1rem' }}>
          <InlineNotification
            kind={scheduleBanner.kind}
            title={scheduleBanner.title}
            subtitle={scheduleBanner.subtitle}
            onClose={() => setScheduleBanner(null)}
          />
        </div>
      )}
      <ComposedModal
        open={scheduleModalOpen}
        onClose={() => {
          if (scheduleSubmitting) return;
          setScheduleModalOpen(false);
          setScheduleModalError('');
        }}
        preventCloseOnClickOutside={scheduleSubmitting}
      >
        <ModalHeader title="Уведомления в Telegram" />
        <ModalBody>
          {scheduleModalError ? (
            <div style={{ marginBottom: '1rem' }}>
              <InlineNotification
                kind="error"
                title="Ошибка"
                subtitle={scheduleModalError}
                lowContrast
                onClose={() => setScheduleModalError('')}
              />
            </div>
          ) : null}
          <p style={{ marginBottom: '1rem' }}>
            Укажите тот же username, что и в боте (без @). Должен быть выполнен /start у бота.
          </p>
          <TextInput
            id="schedule-tg-username"
            labelText="Telegram username"
            placeholder="username"
            value={telegramUsername}
            onChange={(e) => setTelegramUsername(e.target.value)}
            disabled={scheduleSubmitting}
          />
        </ModalBody>
        <ModalFooter>
          <Button
            kind="secondary"
            disabled={scheduleSubmitting}
            onClick={() => {
              setScheduleModalOpen(false);
              setScheduleModalError('');
            }}
          >
            Отмена
          </Button>
          <Button kind="primary" disabled={scheduleSubmitting} onClick={handleScheduleSubmit}>
            {scheduleSubmitting ? 'Отправка…' : 'Отправить'}
          </Button>
        </ModalFooter>
      </ComposedModal>
      <div style={{ marginBottom: '20px' }}>
        <TextInput
          id="time-filter"
          labelText="Фильтр по времени"
          placeholder="Ввведите время (пример, 12:34)"
          value={filterValue}
          onChange={(e) => setFilterValue(e.target.value)}
        />
      </div>

      {filteredData.length > 0 ? (
        <TableContainer title="Scheduled Times">
          <DataTable rows={filteredData} headers={headers} isSortable>
            {({
              rows,
              headers,
              getHeaderProps,
              getRowProps,
              getTableProps,
            }) => (
              <Table {...getTableProps()}>
                <TableHead>
                  <TableRow>
                    {headers.map((header) => (
                      <TableHeader
                        key={header.key}
                        {...getHeaderProps({ header })}
                        onClick={header.key === 'value' ? handleSort : undefined} // Attach sorting to 'value' header
                        style={{ cursor: header.key === 'value' ? 'pointer' : 'default' }}
                      >
                        {header.header}
                      </TableHeader>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={row.id} {...getRowProps({ row })}>
                      {row.cells.map((cell) => (
                        <TableCell key={cell.id}>
                          {cell.value === 'delete' ? (
                            <TrashCan
                              style={{ cursor: 'pointer' }}
                              onClick={() => deleteEntry(row.id)}
                            />
                          ) : (
                            cell.value
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </DataTable>
          <div style={{ marginTop: '20px', display: 'flex', gap: '1rem', flexWrap: 'wrap', alignItems: 'center' }}>
            <Button onClick={downloadJson}>Download JSON</Button>
            <Button onClick={openScheduleModal}>Уведомлять в Telegram</Button>
          </div>
        </TableContainer>
      ) : (
        <div style={{ textAlign: 'center', marginTop: '20px' }}>
          <p>No data found</p>
          <Button onClick={openScheduleModal} style={{ marginTop: '1rem' }}>
            Уведомлять в Telegram
          </Button>
        </div>
      )}
    </div>
  );
};

export default TimeTable;