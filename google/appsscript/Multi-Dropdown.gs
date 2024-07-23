function onEdit(e) {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var activeCell = ss.getActiveCell();
  if (activeCell.getColumn() != 14 || ss.getActiveSheet().getName() != "Sheet1") {
    return;
  }

  var newValue = e.value;
  var oldValue = e.oldValue;
  if (!newValue) {
    activeCell.setValue("");
  }
  else {
    if (!oldValue) {
      activeCell.setValue(newValue);
    } else {
      activeCell.setValue(oldValue + ' ; ' + newValue);
    }
  }
}
