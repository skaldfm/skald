// Tag picker component — searchable multi-select with chips.
// Usage:
//   <div class="tag-picker" data-name="guest_ids"
//        data-items='[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]'
//        data-selected='[1]'></div>
//
// Renders: chips for selected items, search input, dropdown results.
// Submits hidden <input name="guest_ids" value="1"> per selected item.

(function () {
  document.querySelectorAll('.tag-picker').forEach(initPicker);

  function initPicker(container) {
    var fieldName = container.dataset.name;
    var items = JSON.parse(container.dataset.items || '[]') || [];
    var selected = new Set((JSON.parse(container.dataset.selected || '[]') || []).map(Number));

    // Build the DOM
    container.innerHTML = '';

    var chipsWrap = document.createElement('div');
    chipsWrap.className = 'flex flex-wrap gap-1.5 mb-2';
    chipsWrap.style.minHeight = '0';
    container.appendChild(chipsWrap);

    var inputWrap = document.createElement('div');
    inputWrap.className = 'relative';
    container.appendChild(inputWrap);

    var input = document.createElement('input');
    input.type = 'text';
    input.placeholder = 'Search\u2026';
    input.autocomplete = 'off';
    input.className = 'block w-full rounded-md border border-gray-300 dark:border-gray-600 px-3 py-2 shadow-sm ' +
      'focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 dark:bg-gray-700 dark:text-gray-100 text-sm';
    inputWrap.appendChild(input);

    var dropdown = document.createElement('ul');
    dropdown.className = 'absolute z-10 mt-1 max-h-48 w-full overflow-auto rounded-md bg-white dark:bg-gray-700 ' +
      'shadow-lg ring-1 ring-black/5 dark:ring-white/10 hidden';
    inputWrap.appendChild(dropdown);

    // Render functions
    function renderChips() {
      chipsWrap.innerHTML = '';
      // Also clear old hidden inputs
      container.querySelectorAll('input[type="hidden"]').forEach(function (el) { el.remove(); });

      items.forEach(function (item) {
        if (!selected.has(item.id)) return;

        // Hidden form input
        var hidden = document.createElement('input');
        hidden.type = 'hidden';
        hidden.name = fieldName;
        hidden.value = item.id;
        container.appendChild(hidden);

        // Chip
        var chip = document.createElement('span');
        chip.className = 'inline-flex items-center gap-1 rounded-full bg-indigo-50 dark:bg-indigo-900/30 ' +
          'px-2.5 py-1 text-sm font-medium text-indigo-700 dark:text-indigo-400';
        chip.textContent = item.name;

        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'ml-0.5 inline-flex h-4 w-4 items-center justify-center rounded-full ' +
          'text-indigo-400 hover:bg-indigo-200 dark:hover:bg-indigo-800 hover:text-indigo-600 dark:hover:text-indigo-300';
        btn.innerHTML = '<svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
          '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>';
        btn.addEventListener('click', function () {
          selected.delete(item.id);
          renderChips();
          renderDropdown();
        });
        chip.appendChild(btn);
        chipsWrap.appendChild(chip);
      });
    }

    function renderDropdown() {
      var query = input.value.toLowerCase().trim();
      dropdown.innerHTML = '';

      var matches = items.filter(function (item) {
        return !selected.has(item.id) && item.name.toLowerCase().indexOf(query) !== -1;
      });

      if (matches.length === 0 || query === '') {
        dropdown.classList.add('hidden');
        return;
      }

      matches.forEach(function (item) {
        var li = document.createElement('li');
        li.className = 'cursor-pointer px-3 py-2 text-sm text-gray-700 dark:text-gray-200 ' +
          'hover:bg-indigo-50 dark:hover:bg-indigo-900/30 hover:text-indigo-700 dark:hover:text-indigo-400';
        li.textContent = item.name;
        li.addEventListener('mousedown', function (e) {
          e.preventDefault(); // prevent input blur
          selected.add(item.id);
          input.value = '';
          renderChips();
          renderDropdown();
        });
        dropdown.appendChild(li);
      });

      dropdown.classList.remove('hidden');
    }

    input.addEventListener('input', renderDropdown);
    input.addEventListener('focus', function () {
      if (input.value.trim() !== '') renderDropdown();
    });
    input.addEventListener('blur', function () {
      // Delay to allow mousedown on dropdown items
      setTimeout(function () { dropdown.classList.add('hidden'); }, 150);
    });

    // Initial render
    renderChips();
  }
})();
