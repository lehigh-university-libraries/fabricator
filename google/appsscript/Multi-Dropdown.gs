function onEdit(e) {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getActiveSheet();
  if (sheet.getName() != "Sheet1") {
    return;
  }

  var activeCell = ss.getActiveCell();
  var columnNames = ["Related Department", "Language"];
  var columnName = sheet.getRange(1, activeCell.getColumn()).getValue();
  if (!columnNames.includes(columnName)) {
    return;
  }

  var delimiter = ' ; ';
  var newValue = e.value;
  var oldValue = e.oldValue;
  if (!newValue) {
    activeCell.setValue("");
  }
  else {
    if (!oldValue) {
      activeCell.setValue(newValue);
    } else {
      activeCell.setValue(oldValue + delimiter + newValue);
    }
  }
}
